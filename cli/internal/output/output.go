// Package output formats a command result for display, per the freshbooks
// CLI's -o/--output flag: json, yaml, table, or name. See docs/cli.md for
// the user-facing description of each format.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format names one of the four supported output shapes.
type Format string

// The four output formats -o/--output accepts.
const (
	JSON  Format = "json"
	YAML  Format = "yaml"
	Table Format = "table"
	Name  Format = "name"
)

// Valid reports whether f is one of the four supported formats.
func (f Format) Valid() bool {
	switch f {
	case JSON, YAML, Table, Name:
		return true
	default:
		return false
	}
}

// Options controls how Format renders a value.
type Options struct {
	// Format selects json, yaml, table, or name. The zero value is
	// treated as JSON.
	Format Format
	// NoHeaders suppresses the header row in table output. Ignored by
	// every other format.
	NoHeaders bool
}

// DefaultFormat is Table when stdout is a terminal, JSON otherwise -- the
// TTY-sensitive default -o/--output falls back to when unset by a flag,
// env var, or config file (see cli/internal/cmd for the precedence chain).
func DefaultFormat(isTTY bool) Format {
	if isTTY {
		return Table
	}
	return JSON
}

// Write renders value to w per opts. value may be nil, a Go struct, a
// slice, a map, or a json.RawMessage -- anything json.Marshal accepts.
func Write(w io.Writer, value any, opts Options) error {
	raw, err := marshal(value)
	if err != nil {
		return fmt.Errorf("output: marshaling the result: %w", err)
	}

	switch opts.Format {
	case YAML:
		return writeYAML(w, raw)
	case Table:
		return writeTable(w, raw, opts.NoHeaders)
	case Name:
		return writeName(w, raw)
	case JSON, "":
		return writeJSON(w, raw)
	default:
		return fmt.Errorf("output: unknown format %q", opts.Format)
	}
}

// marshal renders value as compact JSON. A nil value marshals to the JSON
// literal null, which every writer below treats as "print nothing of
// substance."
func marshal(value any) (json.RawMessage, error) {
	if raw, ok := value.(json.RawMessage); ok {
		if len(raw) == 0 {
			return json.RawMessage("null"), nil
		}
		return raw, nil
	}
	if value == nil {
		return json.RawMessage("null"), nil
	}
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func isNull(raw json.RawMessage) bool {
	return strings.TrimSpace(string(raw)) == "null"
}

func writeJSON(w io.Writer, raw json.RawMessage) error {
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		// raw came from json.Marshal or a caller-supplied RawMessage;
		// an indent failure means the RawMessage was not valid JSON.
		return fmt.Errorf("output: %w", err)
	}
	buf.WriteByte('\n')
	_, err := w.Write(buf.Bytes())
	return err
}

func writeYAML(w io.Writer, raw json.RawMessage) error {
	v, err := jsonToYAMLValue(raw)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("output: encoding yaml: %w", err)
	}
	_, err = w.Write(b)
	return err
}

// jsonToYAMLValue decodes raw with json.Number preserved, then converts
// every json.Number leaf to an int64 or float64 so yaml.v3 renders plain
// numbers instead of quoted strings, while keeping every object key as the
// snake_case string the JSON form already carries.
func jsonToYAMLValue(raw json.RawMessage) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return convertNumbers(v), nil
}

func convertNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		if f, err := x.Float64(); err == nil {
			return f
		}
		return x.String()
	case map[string]any:
		m := make(map[string]any, len(x))
		for k, vv := range x {
			m[k] = convertNumbers(vv)
		}
		return m
	case []any:
		s := make([]any, len(x))
		for i, vv := range x {
			s[i] = convertNumbers(vv)
		}
		return s
	default:
		return v
	}
}

// rows splits a top-level JSON value into the row set every table/name
// writer shares: a map with an "items" array (the Page[T] wire shape) or a
// bare array yields one row per element; a single object yields one row;
// null yields none.
func rows(raw json.RawMessage) ([]json.RawMessage, error) {
	if isNull(raw) {
		return nil, nil
	}
	trimmed := strings.TrimSpace(string(raw))
	switch {
	case strings.HasPrefix(trimmed, "["):
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return nil, err
		}
		return items, nil
	case strings.HasPrefix(trimmed, "{"):
		var probe struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(raw, &probe); err == nil && probe.Items != nil {
			return probe.Items, nil
		}
		return []json.RawMessage{raw}, nil
	default:
		return []json.RawMessage{raw}, nil
	}
}

func writeTable(w io.Writer, raw json.RawMessage, noHeaders bool) error {
	items, err := rows(raw)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	cols, err := unionKeys(items)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	if len(cols) == 0 {
		// A row that is not a JSON object (a bare scalar list): one
		// unlabeled column.
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		for _, item := range items {
			fmt.Fprintln(tw, cellValue(item)) //nolint:errcheck // tabwriter buffers; Flush reports the real error
		}
		return tw.Flush()
	}

	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if !noHeaders {
		fmt.Fprintln(tw, strings.Join(upper(cols), "\t")) //nolint:errcheck
	}
	for _, item := range items {
		vals, err := decodeFields(item)
		if err != nil {
			return fmt.Errorf("output: %w", err)
		}
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = cellValue(vals[c])
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t")) //nolint:errcheck
	}
	return tw.Flush()
}

func upper(cols []string) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = strings.ToUpper(c)
	}
	return out
}

// nameKeys is the fallback order name output tries for each row.
var nameKeys = []string{"id", "name", "uuid"}

func writeName(w io.Writer, raw json.RawMessage) error {
	items, err := rows(raw)
	if err != nil {
		return fmt.Errorf("output: %w", err)
	}
	for _, item := range items {
		vals, err := decodeFields(item)
		if err != nil {
			return fmt.Errorf("output: %w", err)
		}
		var name string
		for _, k := range nameKeys {
			if v, ok := vals[k]; ok {
				name = cellValue(v)
				break
			}
		}
		fmt.Fprintln(w, name) //nolint:errcheck
	}
	return nil
}

// decodeFields decodes a JSON object into its field values, keyed by name.
// A non-object row decodes to an empty map -- cellValue on the row itself
// handles the "one unlabeled column" case in writeTable.
func decodeFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "{") {
		return map[string]json.RawMessage{}, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// unionKeys returns the table's column set: every key appearing in any of
// items' rows, in order of first appearance across rows in order (F23/
// review A6 -- table output previously derived columns from items[0]
// alone, silently dropping any key a later, heterogeneous row carried
// that the first row did not).
func unionKeys(items []json.RawMessage) ([]string, error) {
	var cols []string
	seen := make(map[string]bool)
	for _, item := range items {
		keys, err := orderedKeys(item)
		if err != nil {
			return nil, err
		}
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				cols = append(cols, k)
			}
		}
	}
	return cols, nil
}

// orderedKeys returns a JSON object's top-level keys in the order they
// appear in raw, using the streaming decoder's token order (map[string]any
// discards it). A non-object value returns nil.
func orderedKeys(raw json.RawMessage) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, nil
	}

	var keys []string
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := keyTok.(string)
		keys = append(keys, key)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// cellValue renders one JSON value as a table/name cell: a bare string
// unquoted, a number or boolean in its original literal form (avoiding a
// float64 round trip that would lose precision on a large integer id), an
// object as "<n keys>", an array as "<n items>", and null/empty as "".
func cellValue(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	switch {
	case s == "" || s == "null":
		return ""
	case strings.HasPrefix(s, `"`):
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			return str
		}
		return s
	case strings.HasPrefix(s, "{"):
		keys, err := orderedKeys(raw)
		if err != nil {
			return "<object>"
		}
		return fmt.Sprintf("<%d keys>", len(keys))
	case strings.HasPrefix(s, "["):
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return "<array>"
		}
		return fmt.Sprintf("<%d items>", len(items))
	default:
		return s
	}
}

// SortedKeys is a small helper for callers (config view) that want a
// stable key order over a map without going through the ordered-JSON path
// above.
func SortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
