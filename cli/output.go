package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"
	"text/template"
)

// validFormat returns true when f is one of the supported output formats.
func validFormat(f string) bool {
	switch f {
	case "table", "json", "jsonl", "csv", "tsv":
		return true
	}
	return false
}

// renderer writes records in a chosen format.
type renderer struct {
	format   string
	noHeader bool
	tmpl     string
	w        io.Writer
}

func newRenderer(w io.Writer, format string, noHeader bool, tmpl string) *renderer {
	return &renderer{format: format, noHeader: noHeader, tmpl: tmpl, w: w}
}

// render writes records (a slice of structs, or a single struct) in the configured format.
func (r *renderer) render(records any) error {
	rv := reflect.ValueOf(records)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		s := reflect.MakeSlice(reflect.SliceOf(rv.Type()), 1, 1)
		s.Index(0).Set(rv)
		rv = s
	}
	n := rv.Len()
	items := make([]any, n)
	for i := 0; i < n; i++ {
		items[i] = rv.Index(i).Interface()
	}

	if r.tmpl != "" {
		return r.renderTemplate(items)
	}
	switch r.format {
	case "json":
		return r.renderJSON(items)
	case "jsonl":
		return r.renderJSONL(items)
	case "csv":
		return r.renderDelimited(items, ',')
	case "tsv":
		return r.renderDelimited(items, '\t')
	default:
		return r.renderTable(items)
	}
}

func (r *renderer) renderJSON(items []any) error {
	enc := json.NewEncoder(r.w)
	enc.SetIndent("", "  ")
	if len(items) == 1 {
		return enc.Encode(items[0])
	}
	return enc.Encode(items)
}

func (r *renderer) renderJSONL(items []any) error {
	enc := json.NewEncoder(r.w)
	for _, item := range items {
		if err := enc.Encode(item); err != nil {
			return err
		}
	}
	return nil
}

func (r *renderer) renderTable(items []any) error {
	if len(items) == 0 {
		return nil
	}
	headers, getters := reflectFields(items[0])
	tw := tabwriter.NewWriter(r.w, 0, 0, 2, ' ', 0)
	if !r.noHeader {
		_, _ = fmt.Fprintln(tw, strings.Join(headers, "\t"))
	}
	for _, item := range items {
		row := make([]string, len(getters))
		for i, g := range getters {
			row[i] = fmt.Sprintf("%v", g(item))
		}
		_, _ = fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

func (r *renderer) renderDelimited(items []any, sep rune) error {
	if len(items) == 0 {
		return nil
	}
	headers, getters := reflectFields(items[0])
	cw := csv.NewWriter(r.w)
	cw.Comma = sep
	if !r.noHeader {
		if err := cw.Write(headers); err != nil {
			return err
		}
	}
	for _, item := range items {
		row := make([]string, len(getters))
		for i, g := range getters {
			row[i] = fmt.Sprintf("%v", g(item))
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func (r *renderer) renderTemplate(items []any) error {
	t, err := template.New("").Parse(r.tmpl)
	if err != nil {
		return fmt.Errorf("template: %w", err)
	}
	for _, item := range items {
		if err := t.Execute(r.w, item); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(r.w)
	}
	return nil
}

// reflectFields returns column headers and value getters derived from json struct tags.
func reflectFields(sample any) (headers []string, getters []func(any) any) {
	t := reflect.TypeOf(sample)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		headers = append(headers, strings.ToUpper(name))
		idx := i
		getters = append(getters, func(v any) any {
			rv := reflect.ValueOf(v)
			if rv.Kind() == reflect.Pointer {
				rv = rv.Elem()
			}
			return rv.Field(idx).Interface()
		})
	}
	return headers, getters
}
