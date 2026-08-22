package main

import (
	"bufio"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec" // #nosec G204 -- args are CLI-supplied Go package patterns (e.g. "./..."), not attacker input
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// CheckOptions configures Check.
type CheckOptions struct {
	// Packages are Go package patterns to scan, e.g. []string{"./..."}.
	Packages []string
	// InventoryPath is the inventory.json produced by Normalize + WriteJSON.
	InventoryPath string
	// IgnorePath is the ignore/todo list; see loadIgnoreList's directive
	// format in the package doc comment on Check.
	IgnorePath string
	// Dir is the working directory `go list` resolves Packages from. Empty
	// uses the current process's working directory; tests point it at a
	// temporary module so the real freshbooks source is never scanned.
	Dir string
}

// CheckReport summarizes one Check run. A report with any Uncovered,
// DoubleCovered, Stale, or Unknown entries represents a failing check; see
// OK.
type CheckReport struct {
	Implemented   int
	Ignored       int
	Todo          int
	Uncovered     []string
	DoubleCovered []string
	Stale         []string
	Unknown       []string
}

// OK reports whether the check passed: every inventory key is implemented,
// ignored, or marked todo, exactly once, with no stray comments.
func (r CheckReport) OK() bool {
	return len(r.Uncovered) == 0 && len(r.DoubleCovered) == 0 && len(r.Stale) == 0 && len(r.Unknown) == 0
}

// String renders the one-line summary printed by the -check CLI mode.
func (r CheckReport) String() string {
	return fmt.Sprintf(
		"implemented %d, ignored %d, todo %d, uncovered %d, double-covered %d, stale %d, unknown %d",
		r.Implemented, r.Ignored, r.Todo,
		len(r.Uncovered), len(r.DoubleCovered), len(r.Stale), len(r.Unknown),
	)
}

// Check scans opts.Packages for `// inventory: <key>` comments and
// cross-references them against opts.InventoryPath and the
// opts.IgnorePath ignore/todo list.
//
// The ignore/todo list format is one directive per line; blank lines and
// "#"-prefixed comment lines are ignored:
//
//	//go:inventory-ignore <key> -- <reason>
//	//go:inventory-todo <key> -- <phase>
//
// Every inventory key must land in exactly one bucket: implemented (one
// `// inventory:` comment, not ignored or todo), ignored, or todo. A key
// with zero coverage is "uncovered"; a key with more than one
// `// inventory:` comment is "double-covered"; a key that is both
// ignored/todo-listed and implemented is "stale"; an `// inventory:`
// comment whose key is not in the inventory at all is "unknown". Any of
// these is a failing check (see CheckReport.OK).
func Check(opts CheckOptions) (CheckReport, error) {
	entries, err := ReadJSON(opts.InventoryPath)
	if err != nil {
		return CheckReport{}, err
	}
	inInventory := make(map[string]bool, len(entries))
	for _, e := range entries {
		inInventory[e.Key] = true
	}

	list, err := loadIgnoreList(opts.IgnorePath)
	if err != nil {
		return CheckReport{}, err
	}
	if err := validateIgnoreList(list, opts.IgnorePath, inInventory); err != nil {
		return CheckReport{}, err
	}

	impls, err := scanPackages(opts.Packages, opts.Dir)
	if err != nil {
		return CheckReport{}, err
	}

	implCount := make(map[string]int)
	for _, im := range impls {
		implCount[im.Key]++
	}

	var report CheckReport
	for key := range implCount {
		if !inInventory[key] {
			report.Unknown = append(report.Unknown, key)
		}
	}

	for _, e := range entries {
		key := e.Key
		count := implCount[key]
		_, ignored := list.Ignore[key]
		_, todo := list.Todo[key]

		if count > 1 {
			report.DoubleCovered = append(report.DoubleCovered, key)
		}
		if (ignored || todo) && count >= 1 {
			report.Stale = append(report.Stale, key)
		}

		switch {
		case count == 1 && !ignored && !todo:
			report.Implemented++
		case count == 0 && ignored:
			report.Ignored++
		case count == 0 && todo:
			report.Todo++
		case count == 0:
			report.Uncovered = append(report.Uncovered, key)
		}
	}

	sort.Strings(report.Uncovered)
	sort.Strings(report.DoubleCovered)
	sort.Strings(report.Stale)
	sort.Strings(report.Unknown)

	return report, nil
}

type implementation struct {
	Key  string
	File string
	Line int
}

var inventoryComment = regexp.MustCompile(`^//\s*inventory:\s*(.+?)\s*$`)

func scanPackages(pkgs []string, dir string) ([]implementation, error) {
	dirs, err := packageDirs(pkgs, dir)
	if err != nil {
		return nil, err
	}

	var impls []implementation
	for _, dir := range dirs {
		if filepath.Base(dir) == "testdata" {
			continue
		}
		des, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("inventory: reading dir %s: %w", dir, err)
		}
		for _, de := range des {
			name := de.Name()
			if de.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			found, err := scanFile(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}
			impls = append(impls, found...)
		}
	}
	return impls, nil
}

// packageDirs resolves Go package patterns to their source directories via
// `go list`, run from dir (or the current directory if dir is empty). pkgs
// are always caller/CLI-supplied package patterns (e.g. "./...",
// "./internal/inventory"), never untrusted input.
func packageDirs(pkgs []string, dir string) ([]string, error) {
	args := append([]string{"list", "-f", "{{.Dir}}"}, pkgs...)
	cmd := exec.Command("go", args...) // #nosec G204
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("inventory: go list %s: %w", strings.Join(pkgs, " "), err)
	}

	var dirs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			dirs = append(dirs, line)
		}
	}
	return dirs, nil
}

func scanFile(path string) ([]implementation, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("inventory: parsing %s: %w", path, err)
	}

	var out []implementation
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			m := inventoryComment.FindStringSubmatch(c.Text)
			if m == nil {
				continue
			}
			pos := fset.Position(c.Pos())
			out = append(out, implementation{Key: m[1], File: path, Line: pos.Line})
		}
	}
	return out, nil
}

type ignoreList struct {
	Ignore map[string]string
	Todo   map[string]string
}

var (
	ignoreDirective = regexp.MustCompile(`^//go:inventory-ignore\s+(.+)$`)
	todoDirective   = regexp.MustCompile(`^//go:inventory-todo\s+(.+)$`)
)

const keyReasonSep = " -- "

func loadIgnoreList(path string) (*ignoreList, error) {
	list := &ignoreList{Ignore: map[string]string{}, Todo: map[string]string{}}

	f, err := os.Open(path) // #nosec G304 -- path is a CLI-supplied ignore-list path, not attacker input
	if err != nil {
		if os.IsNotExist(err) {
			return list, nil
		}
		return nil, fmt.Errorf("inventory: opening %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only fd, close error is not actionable here

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if m := ignoreDirective.FindStringSubmatch(line); m != nil {
			key, reason, err := splitKeyReason(m[1])
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			if err := addUnique(list.Ignore, list.Todo, key, reason, path, lineNo); err != nil {
				return nil, err
			}
			continue
		}
		if m := todoDirective.FindStringSubmatch(line); m != nil {
			key, phase, err := splitKeyReason(m[1])
			if err != nil {
				return nil, fmt.Errorf("%s:%d: %w", path, lineNo, err)
			}
			if err := addUnique(list.Todo, list.Ignore, key, phase, path, lineNo); err != nil {
				return nil, err
			}
			continue
		}
		return nil, fmt.Errorf("%s:%d: unrecognized directive: %q", path, lineNo, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("inventory: scanning %s: %w", path, err)
	}
	return list, nil
}

func addUnique(into, other map[string]string, key, value, path string, lineNo int) error {
	if _, dup := into[key]; dup {
		return fmt.Errorf("%s:%d: key %q listed twice", path, lineNo, key)
	}
	if _, dup := other[key]; dup {
		return fmt.Errorf("%s:%d: key %q listed as both ignore and todo", path, lineNo, key)
	}
	into[key] = value
	return nil
}

func splitKeyReason(rest string) (key, reason string, err error) {
	idx := strings.Index(rest, keyReasonSep)
	if idx < 0 {
		return "", "", fmt.Errorf("expected a %q separator between key and reason: %q", keyReasonSep, rest)
	}
	key = strings.TrimSpace(rest[:idx])
	reason = strings.TrimSpace(rest[idx+len(keyReasonSep):])
	if key == "" {
		return "", "", fmt.Errorf("empty key in %q", rest)
	}
	if reason == "" {
		return "", "", fmt.Errorf("empty reason in %q", rest)
	}
	return key, reason, nil
}

// validateIgnoreList fails loudly if the ignore/todo list references a key
// that is not present in the inventory (a stale or typo'd entry).
func validateIgnoreList(list *ignoreList, path string, inInventory map[string]bool) error {
	var unknown []string
	for key := range list.Ignore {
		if !inInventory[key] {
			unknown = append(unknown, key)
		}
	}
	for key := range list.Todo {
		if !inInventory[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	sort.Strings(unknown)
	return fmt.Errorf("%s: %d key(s) not present in the inventory: %s", path, len(unknown), strings.Join(unknown, "; "))
}
