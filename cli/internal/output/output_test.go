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

func TestSortedKeys(t *testing.T) {
	m := map[string]int{"c": 1, "a": 2, "b": 3}
	got := SortedKeys(m)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	t.Run("[edge] an empty map", func(t *testing.T) {
		if got := SortedKeys(map[string]int{}); len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})
}

func TestConvertNumbers(t *testing.T) {
	t.Run("[happy] a float leaf stays a float64", func(t *testing.T) {
		raw := json.RawMessage(`{"amount": 12.5}`)
		v, err := jsonToYAMLValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]any) //nolint:errcheck
		if _, ok := m["amount"].(float64); !ok {
			t.Errorf("amount = %#v, want float64", m["amount"])
		}
	})

	t.Run("[happy] nested arrays and objects convert recursively", func(t *testing.T) {
		raw := json.RawMessage(`{"items": [{"id": 1}, {"id": 2}], "total": 2}`)
		v, err := jsonToYAMLValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]any)     //nolint:errcheck
		items := m["items"].([]any) //nolint:errcheck
		if len(items) != 2 {
			t.Fatalf("got %d items", len(items))
		}
		first := items[0].(map[string]any) //nolint:errcheck
		if id, ok := first["id"].(int64); !ok || id != 1 {
			t.Errorf("id = %#v", first["id"])
		}
	})

	t.Run("[edge] booleans and strings pass through unchanged", func(t *testing.T) {
		raw := json.RawMessage(`{"active": true, "name": "x"}`)
		v, err := jsonToYAMLValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		m := v.(map[string]any) //nolint:errcheck
		if m["active"] != true || m["name"] != "x" {
			t.Errorf("got %#v", m)
		}
	})
}

func TestWriteTable_Extra(t *testing.T) {
	t.Run("[edge] a bare scalar list gets one unlabeled column", func(t *testing.T) {
		var buf bytes.Buffer
		raw := json.RawMessage(`["a", "b", "c"]`)
		if err := Write(&buf, raw, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		got := buf.String()
		if got != "a\nb\nc\n" {
			t.Errorf("got %q", got)
		}
	})

	t.Run("[edge] a bare number renders as a single row", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, 42, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if strings.TrimSpace(buf.String()) != "42" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("[edge] a string value renders unquoted", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, "hello", Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if strings.TrimSpace(buf.String()) != "hello" {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("[edge] a boolean cell value", func(t *testing.T) {
		var buf bytes.Buffer
		raw := json.RawMessage(`{"active": false}`)
		if err := Write(&buf, raw, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if !strings.Contains(buf.String(), "false") {
			t.Errorf("got %q", buf.String())
		}
	})

	t.Run("[corner] table strips ESC and TAB from a string cell value", func(t *testing.T) {
		// F24/security A1: an API response value carrying a raw ESC
		// (0x1b, the start of an ANSI escape sequence) or an embedded
		// TAB must not reach the terminal or corrupt the tabwriter's own
		// column alignment.
		payload, err := json.Marshal(map[string]string{"name": "probe\x1b[31mred\x1b[0m\tinjected"})
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Write(&buf, json.RawMessage(payload), Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		got := buf.String()
		if strings.ContainsAny(got, "\x1b\t") {
			t.Errorf("got %q, want no ESC or TAB in the rendered output", got)
		}
		if !strings.Contains(got, "probe[31mred[0minjected") {
			t.Errorf("got %q, want the surrounding text preserved with the control characters stripped", got)
		}
	})

	t.Run("[corner] name output also strips control characters", func(t *testing.T) {
		payload, err := json.Marshal(map[string]any{"id": 1, "name": "probe\x1b[31minjected"})
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := Write(&buf, json.RawMessage(payload), Options{Format: Name}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		if strings.Contains(buf.String(), "\x1b") {
			t.Errorf("got %q, want no ESC in name output", buf.String())
		}
	})

	t.Run("[edge] columns are the union of every row's keys, not just the first row's", func(t *testing.T) {
		// F23/review A6: the first row carries only "id"/"name"; the
		// second also carries "email". A column-set derived from row 0
		// alone would silently drop "email" instead of rendering an
		// empty cell for the row that lacks it.
		var buf bytes.Buffer
		raw := json.RawMessage(`[{"id":1,"name":"a"},{"id":2,"name":"b","email":"b@example.com"}]`)
		if err := Write(&buf, raw, Options{Format: Table}); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
		got := buf.String()
		if !strings.Contains(got, "EMAIL") {
			t.Errorf("got %q, want an EMAIL column from row 2", got)
		}
		if !strings.Contains(got, "b@example.com") {
			t.Errorf("got %q, want row 2's email value", got)
		}
		lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("got %d lines, want a header plus 2 rows: %q", len(lines), got)
		}
	})
}

func TestOrderedKeys_NonObject(t *testing.T) {
	keys, err := orderedKeys(json.RawMessage(`[1,2,3]`))
	if err != nil {
		t.Fatalf("orderedKeys() error = %v", err)
	}
	if keys != nil {
		t.Errorf("got %v, want nil for a non-object", keys)
	}
}

func TestWriteName_NonObjectRow(t *testing.T) {
	var buf bytes.Buffer
	raw := json.RawMessage(`["a", "b"]`)
	if err := Write(&buf, raw, Options{Format: Name}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	// Neither element is an object, so decodeFields yields no id/name/uuid
	// for either row -- two empty lines.
	if buf.String() != "\n\n" {
		t.Errorf("got %q", buf.String())
	}
}

func TestWrite_MalformedRawMessage(t *testing.T) {
	// A caller-supplied json.RawMessage that is not actually valid JSON:
	// every format's error path should surface it, not panic.
	bad := json.RawMessage(`{not valid`)

	t.Run("json", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, bad, Options{Format: JSON}); err == nil {
			t.Fatal("Write() error = nil, want an error for malformed JSON")
		}
	})
	t.Run("yaml", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, bad, Options{Format: YAML}); err == nil {
			t.Fatal("Write() error = nil, want an error for malformed JSON")
		}
	})
	t.Run("table", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, bad, Options{Format: Table}); err == nil {
			t.Fatal("Write() error = nil, want an error for malformed JSON")
		}
	})
	t.Run("name", func(t *testing.T) {
		var buf bytes.Buffer
		if err := Write(&buf, bad, Options{Format: Name}); err == nil {
			t.Fatal("Write() error = nil, want an error for malformed JSON")
		}
	})
}

func TestWrite_UnmarshalableValue(t *testing.T) {
	var buf bytes.Buffer
	// A Go channel cannot be marshaled to JSON.
	if err := Write(&buf, make(chan int), Options{Format: JSON}); err == nil {
		t.Fatal("Write() error = nil, want a marshal error")
	}
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
