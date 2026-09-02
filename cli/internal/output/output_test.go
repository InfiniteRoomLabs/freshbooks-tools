package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type sample struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
}

type page struct {
	Items   []sample `json:"items"`
	Page    int      `json:"page"`
	Pages   int      `json:"pages"`
	PerPage int      `json:"per_page"`
	Total   int      `json:"total"`
}

func TestWrite_JSON(t *testing.T) {
	t.Run("[happy] a single struct", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, sample{ID: 1, Name: "a"}, Options{Format: JSON}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		var got sample
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v, got %s", err, buf.String())
		}
		if got.ID != 1 || got.Name != "a" {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("[edge] nil value renders JSON null", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, nil, Options{Format: JSON}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if strings.TrimSpace(buf.String()) != "null" {
			t.Fatalf("got %q, want null", buf.String())
		}
	})

	t.Run("[happy] a json.RawMessage passes through", func(t *testing.T) {
		var buf bytes.Buffer
		raw := json.RawMessage(`{"a":1}`)
		if err := Write(&buf, raw, Options{Format: JSON}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		var got map[string]int
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}
		if got["a"] != 1 {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("[happy] empty string format defaults to json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, sample{ID: 1}, Options{}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if !strings.Contains(buf.String(), `"id": 1`) {
			t.Fatalf("got %s", buf.String())
		}
	})

	t.Run("[sad] unknown format is an error", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, sample{}, Options{Format: "bogus"}); err == nil {
			t.Fatal("Write() error = nil, want an error for an unknown format")
		}
	})
}

func TestWrite_YAML(t *testing.T) {
	t.Run("[happy] keys stay snake_case and numbers stay unquoted", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, sample{ID: 42, Name: "a"}, Options{Format: YAML}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "id: 42") {
			t.Fatalf("got %q, want an unquoted id: 42", out)
		}
		if !strings.Contains(out, "name: a") {
			t.Fatalf("got %q", out)
		}
	})

	t.Run("[edge] a large int64 id keeps its precision", func(t *testing.T) {
		var buf bytes.Buffer
		raw := json.RawMessage(`{"id": 9007199254740993}`)
		if err := Write(&buf, raw, Options{Format: YAML}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if !strings.Contains(buf.String(), "9007199254740993") {
			t.Fatalf("got %q, lost int64 precision", buf.String())
		}
	})

	t.Run("[edge] nil value renders yaml null", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, nil, Options{Format: YAML}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if strings.TrimSpace(buf.String()) != "null" {
			t.Fatalf("got %q", buf.String())
		}
	})
}

func TestWrite_Table(t *testing.T) {
	t.Run("[happy] a Page[T] shape prints one row per item", func(t *testing.T) {
		var buf bytes.Buffer
		p := page{Items: []sample{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}, Page: 1, Pages: 1, PerPage: 2, Total: 2}
		if err := Write(&buf, p, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != 3 { // header + 2 rows
			t.Fatalf("got %d lines, want 3: %q", len(lines), buf.String())
		}
		if !strings.Contains(lines[0], "ID") || !strings.Contains(lines[0], "NAME") {
			t.Fatalf("header = %q", lines[0])
		}
	})

	t.Run("[happy] --no-headers suppresses the header row", func(t *testing.T) {
		var buf bytes.Buffer
		p := page{Items: []sample{{ID: 1, Name: "a"}}}
		if err := Write(&buf, p, Options{Format: Table, NoHeaders: true}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != 1 {
			t.Fatalf("got %d lines, want 1: %q", len(lines), buf.String())
		}
	})

	t.Run("[happy] a single object prints one row", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, sample{ID: 1, Name: "a"}, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2: %q", len(lines), buf.String())
		}
	})

	t.Run("[happy] a bare slice prints one row per element", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, []sample{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("got %d lines, want 3: %q", len(lines), buf.String())
		}
	})

	t.Run("[edge] nested object and array values are elided", func(t *testing.T) {
		var buf bytes.Buffer
		raw := json.RawMessage(`{"id": 1, "meta": {"a":1,"b":2}, "tags": ["x","y","z"]}`)
		if err := Write(&buf, raw, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "<2 keys>") {
			t.Fatalf("got %q, want <2 keys>", out)
		}
		if !strings.Contains(out, "<3 items>") {
			t.Fatalf("got %q, want <3 items>", out)
		}
	})

	t.Run("[edge] nil value prints nothing", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, nil, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("got %q, want empty output", buf.String())
		}
	})

	t.Run("[edge] an empty items page prints nothing", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, page{Items: []sample{}}, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("got %q, want empty output", buf.String())
		}
	})
}

func TestWrite_Name(t *testing.T) {
	t.Run("[happy] falls back id, then name, then uuid", func(t *testing.T) {
		var buf bytes.Buffer
		raw := json.RawMessage(`[{"id": 1}, {"name": "b"}, {"uuid": "c"}, {"other": "d"}]`)
		if err := Write(&buf, raw, Options{Format: Name}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		want := "1\nb\nc\n\n"
		if buf.String() != want {
			t.Fatalf("got %q, want %q", buf.String(), want)
		}
	})

	t.Run("[happy] a single object prints one line", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, sample{ID: 5}, Options{Format: Name}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if strings.TrimSpace(buf.String()) != "5" {
			t.Fatalf("got %q, want 5", buf.String())
		}
	})
}

func TestDefaultFormat(t *testing.T) {
	t.Run("[happy] table on a TTY", func(t *testing.T) {
		if got := DefaultFormat(true); got != Table {
			t.Fatalf("got %v, want table", got)
		}
	})
	t.Run("[happy] json off a TTY", func(t *testing.T) {
		if got := DefaultFormat(false); got != JSON {
			t.Fatalf("got %v, want json", got)
		}
	})
}

func TestFormatValid(t *testing.T) {
	for _, f := range []Format{JSON, YAML, Table, Name} {
		if !f.Valid() {
			t.Errorf("Format(%q).Valid() = false, want true", f)
		}
	}
	if Format("bogus").Valid() {
		t.Error(`Format("bogus").Valid() = true, want false`)
	}
}
