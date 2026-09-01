package tools

import (
	"bufio"
	"os"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
)

// toolsMDPath is docs/phases/3/tools.md relative to this package: the
// definitive, frozen tool surface this registry must implement exactly
// (see docs/phases/3/plan.md, "The definitive tool surface").
const toolsMDPath = "../../../docs/phases/3/tools.md"

type mdRow struct {
	name    string
	service string
	method  string
	keys    []string
}

// tableRowPattern matches one tools.md data row:
// | # | `tool_name` | `Service.Method` (file.go:line) | ANNOT | keys |
var tableRowPattern = regexp.MustCompile("^\\| *\\d+ *\\| *`([a-z0-9_]+)` *\\| *`([A-Za-z]+)\\.([A-Za-z]+)` *\\([^)]*\\) *\\| *[A-Z]+ *\\| *(.+?) *\\|$")

// keyPattern extracts each backtick-quoted key from an mdRow's keys
// column, which lists one or more `Key/Path` entries separated by "; ".
var keyPattern = regexp.MustCompile("`([^`]+)`")

// parseToolsMD reads tools.md's table into one mdRow per tool. A keys
// column of "-" (identity_whoami) yields a nil keys slice.
func parseToolsMD(t *testing.T) []mdRow {
	t.Helper()
	f, err := os.Open(toolsMDPath)
	if err != nil {
		t.Fatalf("opening %s: %v", toolsMDPath, err)
	}
	defer func() { _ = f.Close() }()

	var rows []mdRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := tableRowPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		row := mdRow{name: m[1], service: m[2], method: m[3]}
		if strings.TrimSpace(m[4]) != "-" {
			for _, km := range keyPattern.FindAllStringSubmatch(m[4], -1) {
				row.keys = append(row.keys, km[1])
			}
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading %s: %v", toolsMDPath, err)
	}
	if len(rows) != 168 {
		t.Fatalf("parsed %d rows from %s, want 168", len(rows), toolsMDPath)
	}
	return rows
}

// TestParityAgainstToolsMD asserts the registry implements exactly the
// tool surface docs/phases/3/tools.md defines: same names, same
// (service, method) pairs, same inventory keys per tool.
func TestParityAgainstToolsMD(t *testing.T) {
	rows := parseToolsMD(t)

	byName := make(map[string]mdRow, len(rows))
	for _, r := range rows {
		byName[r.name] = r
	}

	if len(All) != len(rows) {
		t.Fatalf("registry has %d tools, tools.md has %d", len(All), len(rows))
	}

	seen := make(map[string]bool, len(All))
	for _, spec := range All {
		if seen[spec.Name] {
			t.Errorf("duplicate tool name in registry: %s", spec.Name)
		}
		seen[spec.Name] = true

		row, ok := byName[spec.Name]
		if !ok {
			t.Errorf("registry tool %q is not in tools.md", spec.Name)
			continue
		}
		if spec.Service != row.service || spec.Method != row.method {
			t.Errorf("%s: registry wraps %s.%s, tools.md says %s.%s", spec.Name, spec.Service, spec.Method, row.service, row.method)
		}
		if !reflect.DeepEqual(spec.Keys, row.keys) {
			t.Errorf("%s: registry keys %v != tools.md keys %v", spec.Name, spec.Keys, row.keys)
		}
	}
	for name := range byName {
		if !seen[name] {
			t.Errorf("tools.md tool %q is missing from the registry", name)
		}
	}
}

// TestParityAgainstClient reflects over *freshbooks.Client's exported
// service fields and their exported methods (minus every "All" iterator,
// per docs/phases/3/plan.md decision D1) and asserts that set equals the
// registry's (Service, Method) pairs, in both directions.
func TestParityAgainstClient(t *testing.T) {
	clientType := reflect.TypeFor[freshbooks.Client]()

	libPairs := make(map[string]bool)
	for i := range clientType.NumField() {
		field := clientType.Field(i)
		if !field.IsExported() {
			continue
		}
		serviceType := field.Type // a pointer-to-service type
		for m := range serviceType.NumMethod() {
			method := serviceType.Method(m)
			if !method.IsExported() || method.Name == "All" {
				continue
			}
			libPairs[field.Name+"."+method.Name] = true
		}
	}

	registryPairs := make(map[string]string, len(All)) // pair -> tool name
	for _, spec := range All {
		registryPairs[spec.Service+"."+spec.Method] = spec.Name
	}

	for pair := range libPairs {
		if _, ok := registryPairs[pair]; !ok {
			t.Errorf("lib method %s has no tool in the registry", pair)
		}
	}
	for pair, name := range registryPairs {
		if !libPairs[pair] {
			t.Errorf("tool %s (%s) wraps a method the lib does not export (or that is All)", name, pair)
		}
	}
}

// authOwnedKey is Authorization/Revoke Refresh Token, the 213th inventory
// key: it lives on auth.Config.Revoke, not a *freshbooks.Client service
// method, so it is never a tool (docs/phases/3/plan.md decision D2).
const authOwnedKey = "Authorization/Revoke Refresh Token"

// TestParityKeyCoverage asserts the union of every tool's inventory keys
// is exactly the 212 tool-carried keys tools.md documents, each on
// exactly one tool, and that only identity_whoami carries none.
func TestParityKeyCoverage(t *testing.T) {
	rows := parseToolsMD(t)

	var wantKeys []string
	keylessWant := 0
	for _, r := range rows {
		if len(r.keys) == 0 {
			keylessWant++
			continue
		}
		wantKeys = append(wantKeys, r.keys...)
	}
	sort.Strings(wantKeys)

	var gotKeys []string
	keylessGot := 0
	owner := make(map[string]string)
	for _, spec := range All {
		if len(spec.Keys) == 0 {
			keylessGot++
			if spec.Name != "identity_whoami" {
				t.Errorf("%s carries no inventory key; only identity_whoami should", spec.Name)
			}
			continue
		}
		for _, k := range spec.Keys {
			if k == authOwnedKey {
				t.Errorf("%s carries the auth-owned key %q, which must never be a tool key", spec.Name, k)
			}
			if prev, dup := owner[k]; dup {
				t.Errorf("inventory key %q is carried by both %s and %s", k, prev, spec.Name)
			}
			owner[k] = spec.Name
			gotKeys = append(gotKeys, k)
		}
	}
	sort.Strings(gotKeys)

	if keylessGot != 1 || keylessWant != 1 {
		t.Errorf("keyless tools: got %d, want 1 (identity_whoami)", keylessGot)
	}
	if len(gotKeys) != 212 {
		t.Errorf("registry carries %d inventory keys, want 212", len(gotKeys))
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("registry key set != tools.md key set")
	}
}
