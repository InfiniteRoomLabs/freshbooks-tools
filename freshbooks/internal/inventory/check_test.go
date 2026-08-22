package main

import (
	"os"
	"path/filepath"
	"testing"
)

// newFixtureModule creates a standalone Go module under t.TempDir() so
// `go list` can resolve "./..." against it without touching the real
// freshbooks module. files maps a relative path (e.g. "impl.go") to its
// content.
func newFixtureModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	mod := "module fixture\n\ngo 1.26\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0o600); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}
	for name, content := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

func writeInventoryFixture(t *testing.T, keys ...string) string {
	t.Helper()
	entries := make([]Entry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, Entry{
			Key: k, Folder: "Folder", Path: []string{}, Name: k, Method: "GET",
			PathTemplate: "/x", Host: "api.freshbooks.com", Query: []QueryEntry{},
			Responses: []RespEntry{}, Family: FamilyAccounting, Duplicates: 1,
		})
	}
	path := filepath.Join(t.TempDir(), "inventory.json")
	if err := WriteJSON(path, entries); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	return path
}

func writeIgnoreFixture(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ignore.list")
	content := ""
	for _, l := range lines {
		content += l + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing ignore list: %v", err)
	}
	return path
}

func TestCheckImplementedPass(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc ListClients() {}\n",
	})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t)

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !report.OK() {
		t.Errorf("report.OK() = false, report = %+v", report)
	}
	if report.Implemented != 1 {
		t.Errorf("Implemented = %d, want 1", report.Implemented)
	}
}

func TestCheckIgnoredPass(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl.go": "package fixture\n",
	})
	inv := writeInventoryFixture(t, "Uploader/Internal Thing")
	ignore := writeIgnoreFixture(t, "//go:inventory-ignore Uploader/Internal Thing -- internal my.freshbooks.com endpoint")

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !report.OK() || report.Ignored != 1 {
		t.Errorf("report = %+v, want OK with Ignored=1", report)
	}
}

func TestCheckTodoPass(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl.go": "package fixture\n",
	})
	inv := writeInventoryFixture(t, "Invoices/Send Invoice by Email")
	ignore := writeIgnoreFixture(t, "//go:inventory-todo Invoices/Send Invoice by Email -- phase-2")

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !report.OK() || report.Todo != 1 {
		t.Errorf("report = %+v, want OK with Todo=1", report)
	}
}

func TestCheckUncoveredFails(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{"impl.go": "package fixture\n"})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t)

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.OK() {
		t.Fatal("report.OK() = true, want false for an uncovered key")
	}
	if len(report.Uncovered) != 1 || report.Uncovered[0] != "Clients/List Clients" {
		t.Errorf("Uncovered = %v", report.Uncovered)
	}
}

func TestCheckDoubleCoveredFails(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"a.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc A() {}\n",
		"b.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc B() {}\n",
	})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t)

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.OK() {
		t.Fatal("report.OK() = true, want false for a double-covered key")
	}
	if len(report.DoubleCovered) != 1 || report.DoubleCovered[0] != "Clients/List Clients" {
		t.Errorf("DoubleCovered = %v", report.DoubleCovered)
	}
}

func TestCheckStaleFails(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc A() {}\n",
	})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t, "//go:inventory-todo Clients/List Clients -- phase-2")

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.OK() {
		t.Fatal("report.OK() = true, want false for a stale todo entry")
	}
	if len(report.Stale) != 1 || report.Stale[0] != "Clients/List Clients" {
		t.Errorf("Stale = %v", report.Stale)
	}
}

func TestCheckUnknownCommentFails(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl.go": "package fixture\n\n// inventory: Nonexistent/Key\nfunc A() {}\n",
	})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t, "//go:inventory-todo Clients/List Clients -- phase-2")

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if report.OK() {
		t.Fatal("report.OK() = true, want false for an unknown comment key")
	}
	if len(report.Unknown) != 1 || report.Unknown[0] != "Nonexistent/Key" {
		t.Errorf("Unknown = %v", report.Unknown)
	}
}

func TestCheckIgnoresTestFilesAndTestdata(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl_test.go":     "package fixture\n\n// inventory: Clients/List Clients\nfunc TestA(t interface{}) {}\n",
		"testdata/impl.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc A() {}\n",
		"real.go":          "package fixture\n",
	})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t)

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	// Both candidate comments live in excluded locations (_test.go and
	// testdata/), so the key must still read as uncovered.
	if report.OK() {
		t.Fatal("report.OK() = true, want false: _test.go and testdata/ comments must not count")
	}
	if len(report.Uncovered) != 1 {
		t.Errorf("Uncovered = %v, want [Clients/List Clients]", report.Uncovered)
	}
}

func TestLoadIgnoreListMalformedLine(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{"impl.go": "package fixture\n"})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t, "//go:inventory-ignore Clients/List Clients no separator here")

	if _, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir}); err == nil {
		t.Fatal("Check() error = nil, want an error for a malformed ignore-list line")
	}
}

func TestLoadIgnoreListDuplicateKey(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{"impl.go": "package fixture\n"})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t,
		"//go:inventory-ignore Clients/List Clients -- first reason",
		"//go:inventory-ignore Clients/List Clients -- second reason",
	)

	if _, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir}); err == nil {
		t.Fatal("Check() error = nil, want an error for a key listed twice")
	}
}

func TestLoadIgnoreListKeyInBothListsFails(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{"impl.go": "package fixture\n"})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t,
		"//go:inventory-ignore Clients/List Clients -- reason",
		"//go:inventory-todo Clients/List Clients -- phase-2",
	)

	if _, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir}); err == nil {
		t.Fatal("Check() error = nil, want an error for a key listed as both ignore and todo")
	}
}

func TestLoadIgnoreListUnknownKeyFails(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{"impl.go": "package fixture\n"})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t, "//go:inventory-ignore Nonexistent/Key -- reason")

	if _, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir}); err == nil {
		t.Fatal("Check() error = nil, want an error for an ignore-list key absent from the inventory")
	}
}

func TestLoadIgnoreListMissingFileIsEmpty(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{
		"impl.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc A() {}\n",
	})
	inv := writeInventoryFixture(t, "Clients/List Clients")

	report, err := Check(CheckOptions{
		Packages:      []string{"./..."},
		InventoryPath: inv,
		IgnorePath:    filepath.Join(t.TempDir(), "does-not-exist.list"),
		Dir:           dir,
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !report.OK() {
		t.Errorf("report = %+v, want OK (missing ignore file treated as empty)", report)
	}
}

func TestLoadIgnoreListCommentsAndBlankLines(t *testing.T) {
	dir := newFixtureModule(t, map[string]string{"impl.go": "package fixture\n"})
	inv := writeInventoryFixture(t, "Clients/List Clients")
	ignore := writeIgnoreFixture(t,
		"# a comment",
		"",
		"//go:inventory-todo Clients/List Clients -- phase-2",
	)

	report, err := Check(CheckOptions{Packages: []string{"./..."}, InventoryPath: inv, IgnorePath: ignore, Dir: dir})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !report.OK() || report.Todo != 1 {
		t.Errorf("report = %+v, want OK with Todo=1", report)
	}
}

func TestCheckReportString(t *testing.T) {
	r := CheckReport{Implemented: 1, Ignored: 2, Todo: 3, Uncovered: []string{"a"}, DoubleCovered: []string{"b"}, Stale: []string{"c"}, Unknown: []string{"d"}}
	want := "implemented 1, ignored 2, todo 3, uncovered 1, double-covered 1, stale 1, unknown 1"
	if got := r.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
