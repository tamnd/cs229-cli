// Package cs229 is the library behind the cs229 command line:
// the HTTP client, request shaping, and typed data models for the
// Stanford CS229 Machine Learning course.
package cs229

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to the course website.
const DefaultUserAgent = "cs229/dev (+https://github.com/tamnd/cs229-cli)"

// DefaultSemester is the semester used when none is provided.
const DefaultSemester = "fall25"

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://cs229.stanford.edu",
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the CS229 course website over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	last time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Lecture is a single lecture row from the schedule.
type Lecture struct {
	Number   int    `json:"number"`
	Date     string `json:"date"`
	Session  string `json:"session"`
	Topic    string `json:"topic"`
	Details  string `json:"details"`
	Notes    string `json:"notes"`
	URL      string `json:"url"`
	Semester string `json:"semester"`
}

// semesterURL maps a semester key to its page URL on cs229.stanford.edu.
// Newer pages use index.html-* naming; older semesters use syllabus-*.html.
func (c *Client) semesterURL(semester string) string {
	// Known newer-format pages
	newStyle := map[string]string{
		"win26":       "index.html-win26",
		"fall25":      "index.html-fall25",
		"summer25":    "index.html-summer25",
		"winter25":    "w24-index.html",
		"fall24":      "index.html.fall24_prev",
		"summer24":    "index.html-backup-summer24",
		"winter24":    "index.html_winter_2024_backup",
		"fall23":      "index.html-backup-fall23",
		"summer23":    "index.html-backup-summer23",
		"spring23":    "2023_index.html",
	}
	if path, ok := newStyle[semester]; ok {
		return c.cfg.BaseURL + "/" + path
	}
	// Older semesters: syllabus-<semester>.html
	return c.cfg.BaseURL + "/syllabus-" + semester + ".html"
}

// Lectures fetches the schedule page for the given semester and returns
// parsed lecture rows. semester defaults to DefaultSemester when empty.
// limit <= 0 returns all.
func (c *Client) Lectures(ctx context.Context, semester string, limit int) ([]Lecture, error) {
	if semester == "" {
		semester = DefaultSemester
	}
	rawURL := c.semesterURL(semester)
	body, err := c.get(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("lectures %s: %w", semester, err)
	}
	lectures := parseSchedule(string(body), semester, c.cfg.BaseURL)
	if limit > 0 && limit < len(lectures) {
		lectures = lectures[:limit]
	}
	return lectures, nil
}

// Search filters lectures whose Session, Topic, or Details contains query
// (case-insensitive). semester defaults to DefaultSemester when empty.
// limit <= 0 returns all matches.
func (c *Client) Search(ctx context.Context, query, semester string, limit int) ([]Lecture, error) {
	all, err := c.Lectures(ctx, semester, 0)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var out []Lecture
	for _, l := range all {
		if strings.Contains(strings.ToLower(l.Topic), q) ||
			strings.Contains(strings.ToLower(l.Session), q) ||
			strings.Contains(strings.ToLower(l.Details), q) {
			out = append(out, l)
			if limit > 0 && len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// ─── HTTP ─────────────────────────────────────────────────────────────────────

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,*/*")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// ─── HTML parsing ─────────────────────────────────────────────────────────────

// parseSchedule detects which HTML format the page uses and delegates.
// It uses only stdlib strings methods — no golang.org/x/net/html.
func parseSchedule(html, semester, baseURL string) []Lecture {
	// Newer format: <section id="schedule"> with Date/Session/Topic/Details
	if strings.Contains(html, `id="schedule"`) && strings.Contains(html, "<th>Session</th>") {
		return parseNewSchedule(html, semester, baseURL)
	}
	// Older format: <table id="schedule"> with Event/Date/Description/Materials
	if strings.Contains(html, `id="schedule"`) {
		return parseOldSchedule(html, semester, baseURL)
	}
	// Fallback: try new format anyway
	return parseNewSchedule(html, semester, baseURL)
}

// parseNewSchedule handles the newer index.html-* pages (fall25 style).
// Columns: Date | Session | Topic | Details
func parseNewSchedule(html, semester, baseURL string) []Lecture {
	// Find the schedule section
	schedIdx := strings.Index(html, `id="schedule"`)
	if schedIdx == -1 {
		schedIdx = 0
	}
	// Find tbody
	tbodyStart := strings.Index(html[schedIdx:], "<tbody>")
	if tbodyStart == -1 {
		return nil
	}
	tbodyStart += schedIdx
	tbodyEnd := strings.Index(html[tbodyStart:], "</tbody>")
	if tbodyEnd == -1 {
		tbodyEnd = len(html) - tbodyStart
	}
	tbody := html[tbodyStart : tbodyStart+tbodyEnd]

	var lectures []Lecture
	lectureNum := 0
	rows := splitTR(tbody)
	for _, row := range rows {
		if strings.Contains(row, "<th") {
			continue
		}
		cells := extractTDs(row)
		if len(cells) < 3 {
			continue
		}
		date := cleanText(stripHTMLTags(cells[0]))
		session := cleanText(stripHTMLTags(cells[1]))
		topic := cleanText(stripHTMLTags(cells[2]))
		details := ""
		if len(cells) >= 4 {
			details = cleanText(stripHTMLTags(cells[3]))
		}

		if date == "" || session == "" {
			continue
		}
		// Skip non-session rows (project reports, poster sessions, etc.)
		if topic == "" && !strings.Contains(strings.ToLower(session), "lecture") {
			continue
		}

		pageURL := baseURL + "/" + "index.html-" + semester + "#schedule"

		lectureNum++
		lectures = append(lectures, Lecture{
			Number:   lectureNum,
			Date:     date,
			Session:  session,
			Topic:    topic,
			Details:  details,
			Semester: semester,
			URL:      pageURL,
		})
	}
	return lectures
}

// parseOldSchedule handles the older syllabus-*.html pages (2018-2022 style).
// Columns: Event | Date | Description | Materials and Assignments
func parseOldSchedule(html, semester, baseURL string) []Lecture {
	// Find the schedule table
	schedIdx := strings.Index(html, `id="schedule"`)
	if schedIdx == -1 {
		return nil
	}
	tbodyStart := strings.Index(html[schedIdx:], "<tbody>")
	if tbodyStart == -1 {
		// Try without tbody
		tbodyStart = strings.Index(html[schedIdx:], "<tr")
		if tbodyStart == -1 {
			return nil
		}
	}
	tbodyStart += schedIdx
	tbodyEnd := strings.Index(html[tbodyStart:], "</table>")
	if tbodyEnd == -1 {
		tbodyEnd = len(html) - tbodyStart
	}
	tbody := html[tbodyStart : tbodyStart+tbodyEnd]

	var lectures []Lecture
	lectureNum := 0
	rows := splitTR(tbody)
	for _, row := range rows {
		if strings.Contains(row, "<th") {
			continue
		}
		cells := extractTDs(row)
		if len(cells) < 3 {
			continue
		}

		event := cleanText(stripHTMLTags(cells[0]))
		date := cleanText(stripHTMLTags(cells[1]))
		description := cleanText(stripHTMLTags(cells[2]))
		materialsHTML := ""
		materials := ""
		if len(cells) >= 4 {
			materialsHTML = cells[3]
			materials = cleanText(stripHTMLTags(materialsHTML))
		}

		// Only include rows that look like lectures
		eventLower := strings.ToLower(event)
		if !strings.HasPrefix(eventLower, "lecture") {
			continue
		}
		if description == "" {
			continue
		}

		// Extract notes links from the raw HTML of the materials column
		notes := extractLinks(materialsHTML)

		pageURL := baseURL + "/syllabus-" + semester + ".html"

		lectureNum++
		lectures = append(lectures, Lecture{
			Number:   lectureNum,
			Date:     date,
			Session:  event,
			Topic:    description,
			Details:  materials,
			Notes:    notes,
			Semester: semester,
			URL:      pageURL,
		})
	}
	return lectures
}

// extractLinks collects the first href= found in an HTML snippet.
func extractLinks(html string) string {
	idx := strings.Index(html, `href="`)
	if idx == -1 {
		return ""
	}
	start := idx + 6
	end := strings.Index(html[start:], `"`)
	if end == -1 {
		return ""
	}
	return html[start : start+end]
}

// splitTR returns the content of each <tr>...</tr> block.
func splitTR(html string) []string {
	var rows []string
	for {
		start := strings.Index(html, "<tr")
		if start == -1 {
			break
		}
		end := strings.Index(html[start:], "</tr>")
		if end == -1 {
			break
		}
		rows = append(rows, html[start:start+end+5])
		html = html[start+end+5:]
	}
	return rows
}

// extractTDs returns the innerHTML of each <td>...</td> in a row.
// It handles rowspan/colspan by returning what is actually there.
func extractTDs(row string) []string {
	var cells []string
	rest := row
	for {
		start := strings.Index(rest, "<td")
		if start == -1 {
			break
		}
		// skip to end of opening tag
		openEnd := strings.Index(rest[start:], ">")
		if openEnd == -1 {
			break
		}
		content := rest[start+openEnd+1:]
		end := strings.Index(content, "</td>")
		if end == -1 {
			break
		}
		cells = append(cells, content[:end])
		rest = content[end+5:]
	}
	return cells
}

// stripHTMLTags removes all HTML tags and decodes common entities.
func stripHTMLTags(s string) string {
	var b strings.Builder
	inTag := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '<':
			inTag = true
		case ch == '>':
			inTag = false
			b.WriteByte(' ')
		case !inTag:
			b.WriteByte(ch)
		}
	}
	out := b.String()
	out = strings.ReplaceAll(out, "&amp;", "&")
	out = strings.ReplaceAll(out, "&lt;", "<")
	out = strings.ReplaceAll(out, "&gt;", ">")
	out = strings.ReplaceAll(out, "&quot;", `"`)
	out = strings.ReplaceAll(out, "&#39;", "'")
	out = strings.ReplaceAll(out, "&apos;", "'")
	out = strings.ReplaceAll(out, "&mdash;", "-")
	out = strings.ReplaceAll(out, "&ndash;", "-")
	out = strings.ReplaceAll(out, "&nbsp;", " ")
	return strings.TrimSpace(out)
}

// cleanText normalises whitespace in a string.
func cleanText(s string) string {
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
