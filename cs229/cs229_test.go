package cs229_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/cs229-cli/cs229"
)

// minScheduleNewHTML is a minimal excerpt of the newer index.html-* schedule format.
const minScheduleNewHTML = `<!DOCTYPE html>
<html>
<body>
<section id="schedule">
  <div class="container">
    <h2>Course Schedule</h2>
    <table class="table table-striped">
      <thead>
        <tr>
          <th>Date</th>
          <th>Session</th>
          <th>Topic</th>
          <th>Details</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td>September 22, 2025</td>
          <td>Lecture 1</td>
          <td>Introduction</td>
          <td>Problem Set 0 Released</td>
        </tr>
        <tr>
          <td>September 24, 2025</td>
          <td>TA Lecture 1</td>
          <td>Linear Algebra Review</td>
          <td></td>
        </tr>
        <tr>
          <td>September 29, 2025</td>
          <td>Lecture 3</td>
          <td>Weighted Least Squares. Logistic regression.</td>
          <td></td>
        </tr>
        <tr style="font-weight: bold; color: red;">
          <td>October 30, 2025</td>
          <td>MIDTERM</td>
          <td>MIDTERM EXAM</td>
          <td>Location TBD</td>
        </tr>
      </tbody>
    </table>
  </div>
</section>
</body>
</html>`

// minScheduleOldHTML is a minimal excerpt of the older syllabus-*.html schedule format.
const minScheduleOldHTML = `<!DOCTYPE html>
<html>
<body>
<table id="schedule" class="table table-bordered">
  <thead class="active">
    <th>Event</th><th>Date</th><th>Description</th><th>Materials and Assignments</th>
  </thead>
  <tbody>
  <tr>
    <td>Lecture&nbsp;1</td>
    <td> 9/24 </td>
    <td>Introduction and Basic Concepts</td>
    <td></td>
  </tr>
  <tr style="text-align:center; background-color:#FFF2F2">
    <td>A0</td>
    <td>9/24</td>
    <td colspan="3">Problem Set 0. Out 9/24. Due 10/3.</td>
  </tr>
  <tr>
    <td>Lecture&nbsp;2</td>
    <td>9/26</td>
    <td>Supervised Learning Setup. Linear Regression.</td>
    <td>
      <strong>Class Notes</strong>
      <ul>
      <li>Supervised Learning [<a href="http://cs229.stanford.edu/notes/cs229-notes1.pdf">pdf</a>]</li>
      </ul>
    </td>
  </tr>
  <tr>
    <td>Section</td>
    <td>9/28</td>
    <td colspan="2">Discussion Section: Linear Algebra</td>
  </tr>
  </tbody>
</table>
</body>
</html>`

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request missing User-Agent")
		}
		_, _ = w.Write([]byte(body))
	}))
}

func newTestClient(baseURL string) *cs229.Client {
	cfg := cs229.DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	cfg.Retries = 0
	cfg.Timeout = 5 * time.Second
	return cs229.NewClient(cfg)
}

func TestLecturesNewFormat(t *testing.T) {
	srv := newTestServer(t, minScheduleNewHTML)
	defer srv.Close()

	c := newTestClient(srv.URL)
	lectures, err := c.Lectures(context.Background(), "fall25", 0)
	if err != nil {
		t.Fatalf("Lectures: %v", err)
	}

	// Should include all 4 rows (Introduction, TA Lecture, Logistic regression, MIDTERM)
	if len(lectures) < 3 {
		t.Fatalf("got %d lectures, want at least 3", len(lectures))
	}

	l0 := lectures[0]
	if !strings.Contains(l0.Topic, "Introduction") {
		t.Errorf("lecture 0 topic = %q, want Introduction", l0.Topic)
	}
	if l0.Date != "September 22, 2025" {
		t.Errorf("lecture 0 date = %q, want September 22, 2025", l0.Date)
	}
	if l0.Session != "Lecture 1" {
		t.Errorf("lecture 0 session = %q, want Lecture 1", l0.Session)
	}
	if l0.Details != "Problem Set 0 Released" {
		t.Errorf("lecture 0 details = %q, want Problem Set 0 Released", l0.Details)
	}
	if l0.Semester != "fall25" {
		t.Errorf("lecture 0 semester = %q, want fall25", l0.Semester)
	}
}

func TestLecturesOldFormat(t *testing.T) {
	srv := newTestServer(t, minScheduleOldHTML)
	defer srv.Close()

	c := newTestClient(srv.URL)
	lectures, err := c.Lectures(context.Background(), "autumn2018", 0)
	if err != nil {
		t.Fatalf("Lectures: %v", err)
	}

	// Should have 2 lectures (A0 and Section rows excluded)
	if len(lectures) != 2 {
		t.Fatalf("got %d lectures, want 2", len(lectures))
	}

	l0 := lectures[0]
	if !strings.Contains(l0.Topic, "Introduction") {
		t.Errorf("lecture 0 topic = %q, want Introduction", l0.Topic)
	}
	if l0.Semester != "autumn2018" {
		t.Errorf("lecture 0 semester = %q, want autumn2018", l0.Semester)
	}

	l1 := lectures[1]
	if !strings.Contains(l1.Topic, "Supervised Learning") {
		t.Errorf("lecture 1 topic = %q, want Supervised Learning", l1.Topic)
	}
	// Lecture 2 should have notes URL extracted
	if l1.Notes == "" {
		t.Error("lecture 1 notes URL is empty, want a PDF link")
	}
}

func TestLecturesLimit(t *testing.T) {
	srv := newTestServer(t, minScheduleNewHTML)
	defer srv.Close()

	c := newTestClient(srv.URL)
	lectures, err := c.Lectures(context.Background(), "fall25", 1)
	if err != nil {
		t.Fatalf("Lectures: %v", err)
	}
	if len(lectures) != 1 {
		t.Fatalf("got %d lectures with limit 1, want 1", len(lectures))
	}
}

func TestSearch(t *testing.T) {
	srv := newTestServer(t, minScheduleNewHTML)
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "logistic", "fall25", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if !strings.Contains(results[0].Topic, "Logistic") {
		t.Errorf("result topic = %q, want Logistic", results[0].Topic)
	}
}

func TestSearchNoMatch(t *testing.T) {
	srv := newTestServer(t, minScheduleNewHTML)
	defer srv.Close()

	c := newTestClient(srv.URL)
	results, err := c.Search(context.Background(), "zzznomatch", "fall25", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("got %d results for no-match query, want 0", len(results))
	}
}

func TestRetryOn503(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(minScheduleNewHTML))
	}))
	defer srv.Close()

	cfg := cs229.DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	cfg.Timeout = 10 * time.Second
	c := cs229.NewClient(cfg)

	_, err := c.Lectures(context.Background(), "fall25", 0)
	if err != nil {
		t.Fatalf("expected retry to succeed: %v", err)
	}
	if hits < 3 {
		t.Errorf("expected at least 3 hits, got %d", hits)
	}
}

func TestDefaultSemester(t *testing.T) {
	srv := newTestServer(t, minScheduleNewHTML)
	defer srv.Close()

	c := newTestClient(srv.URL)
	// empty semester should default to fall25
	lectures, err := c.Lectures(context.Background(), "", 0)
	if err != nil {
		t.Fatalf("Lectures with empty semester: %v", err)
	}
	if len(lectures) == 0 {
		t.Error("expected lectures with default semester")
	}
	if lectures[0].Semester != cs229.DefaultSemester {
		t.Errorf("semester = %q, want %q", lectures[0].Semester, cs229.DefaultSemester)
	}
}
