package cmd

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

// commandsMDPath is docs/phases/4/commands.md relative to this package:
// the definitive, frozen 168-row command surface this registry must
// implement exactly in name, lib method, and inventory keys (the flags
// column is a heuristic the lib signature is allowed to override -- see
// the implementer report for every row where it does).
const commandsMDPath = "../../../docs/phases/4/commands.md"

type mdRow struct {
	command string // "freshbooks <group> <verb>"
	service string
	method  string
	annot   string
	keys    []string
}

// commandRowPattern matches one commands.md data row:
// | # | `freshbooks <group> <verb>` | `Service.Method` | ANNOT | flags | keys |
var commandRowPattern = regexp.MustCompile("^\\| *\\d+ *\\| *`([a-z0-9 -]+)` *\\| *`([A-Za-z]+)\\.([A-Za-z]+)` *\\| *([A-Z]+) *\\|.*\\| *(.+?) *\\|$")

// keyPattern extracts each backtick-quoted key from an mdRow's keys
// column, which lists one or more `Key/Path` entries separated by "; ".
var keyPattern = regexp.MustCompile("`([^`]+)`")

func parseCommandsMD(t *testing.T) []mdRow {
	t.Helper()
	f, err := os.Open(commandsMDPath)
	if err != nil {
		t.Fatalf("opening %s: %v", commandsMDPath, err)
	}
	defer func() { _ = f.Close() }()

	var rows []mdRow
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := commandRowPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		row := mdRow{command: strings.TrimSpace(m[1]), service: m[2], method: m[3], annot: m[4]}
		if strings.TrimSpace(m[5]) != "-" {
			for _, km := range keyPattern.FindAllStringSubmatch(m[5], -1) {
				row.keys = append(row.keys, km[1])
			}
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading %s: %v", commandsMDPath, err)
	}
	if len(rows) != 168 {
		t.Fatalf("parsed %d rows from %s, want 168", len(rows), commandsMDPath)
	}
	return rows
}

// TestParityAgainstCommandsMD asserts the registry implements exactly the
// command surface docs/phases/4/commands.md defines: same "freshbooks
// <group> <verb>" paths, same (service, method) pairs, same inventory
// keys per command.
func TestParityAgainstCommandsMD(t *testing.T) {
	rows := parseCommandsMD(t)

	byCommand := make(map[string]mdRow, len(rows))
	for _, r := range rows {
		byCommand[r.command] = r
	}

	if len(All) != len(rows) {
		t.Fatalf("registry has %d commands, commands.md has %d", len(All), len(rows))
	}

	seen := make(map[string]bool, len(All))
	for _, c := range All {
		path := "freshbooks " + c.Group + " " + c.Verb
		if seen[path] {
			t.Errorf("duplicate command in registry: %s", path)
		}
		seen[path] = true

		row, ok := byCommand[path]
		if !ok {
			t.Errorf("registry command %q is not in commands.md", path)
			continue
		}
		if c.Service != row.service || c.Method != row.method {
			t.Errorf("%s: registry wraps %s.%s, commands.md says %s.%s", path, c.Service, c.Method, row.service, row.method)
		}
		if !reflect.DeepEqual([]string(c.Keys), row.keys) {
			t.Errorf("%s: registry keys %v != commands.md keys %v", path, c.Keys, row.keys)
		}
		if got := annotClass(c.Class); got != row.annot {
			t.Errorf("%s: registry annotation class %q != commands.md column %q", path, got, row.annot)
		}
	}
	for path := range byCommand {
		if !seen[path] {
			t.Errorf("commands.md command %q is missing from the registry", path)
		}
	}
}

func annotClass(c Class) string { return string(c) }

// TestParityAgainstClient reflects over *freshbooks.Client's exported
// service fields and their exported methods (minus every "All" iterator,
// per D1/D2) and asserts that set equals the registry's (Service, Method)
// pairs, in both directions.
func TestParityAgainstClient(t *testing.T) {
	clientType := reflect.TypeFor[freshbooks.Client]()

	libPairs := make(map[string]bool)
	allIterators := make(map[string]bool) // "Service" -> has an All method
	for i := range clientType.NumField() {
		field := clientType.Field(i)
		if !field.IsExported() {
			continue
		}
		serviceType := field.Type // a pointer-to-service type
		for m := range serviceType.NumMethod() {
			method := serviceType.Method(m)
			if !method.IsExported() {
				continue
			}
			if method.Name == "All" {
				allIterators[field.Name] = true
				continue
			}
			libPairs[field.Name+"."+method.Name] = true
		}
	}

	registryPairs := make(map[string]string, len(All)) // pair -> "group verb"
	hasAllByService := make(map[string]bool)
	for _, c := range All {
		registryPairs[c.Service+"."+c.Method] = c.Group + " " + c.Verb
		if c.HasAll {
			hasAllByService[c.Service] = true
		}
	}

	for pair := range libPairs {
		if _, ok := registryPairs[pair]; !ok {
			t.Errorf("lib method %s has no command in the registry", pair)
		}
	}
	for pair, name := range registryPairs {
		if !libPairs[pair] {
			t.Errorf("command %q (%s) wraps a method the lib does not export (or that is All)", name, pair)
		}
	}

	// Every service with an All iterator has its list command flagged
	// --all.
	for service := range allIterators {
		if !hasAllByService[service] {
			t.Errorf("service %s has an All iterator but no registry command sets HasAll", service)
		}
	}
}

// authOwnedKey is Authorization/Revoke Refresh Token, the 213th inventory
// key: it lives on auth.Config.Revoke, not a *freshbooks.Client service
// method, so it is never a registry command.
const authOwnedKey = "Authorization/Revoke Refresh Token"

// TestParityKeyCoverage asserts the union of every command's inventory
// keys is exactly the 212 command-carried keys commands.md documents,
// each on exactly one command, and that only identity_whoami carries
// none.
func TestParityKeyCoverage(t *testing.T) {
	rows := parseCommandsMD(t)

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
	for _, c := range All {
		path := c.Group + " " + c.Verb
		if len(c.Keys) == 0 {
			keylessGot++
			if path != "identity whoami" {
				t.Errorf("%s carries no inventory key; only identity whoami should", path)
			}
			continue
		}
		for _, k := range c.Keys {
			if k == authOwnedKey {
				t.Errorf("%s carries the auth-owned key %q, which must never be a command key", path, k)
			}
			if prev, dup := owner[k]; dup {
				t.Errorf("inventory key %q is carried by both %s and %s", k, prev, path)
			}
			owner[k] = path
			gotKeys = append(gotKeys, k)
		}
	}
	sort.Strings(gotKeys)

	if keylessGot != 1 || keylessWant != 1 {
		t.Errorf("keyless commands: got %d, want 1 (identity whoami)", keylessGot)
	}
	if len(gotKeys) != 212 {
		t.Errorf("registry carries %d inventory keys, want 212", len(gotKeys))
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("registry key set != commands.md key set")
	}
}

// nonRegistryGroups are the cobra parent commands with subcommands of
// their own that are NOT part of the 168-command registry (D1's
// "non-registry commands": auth, config). BuildTree only ever creates a
// parent group for a registry entry's Command.Group, so these two names
// can never collide with a real resource group.
var nonRegistryGroups = map[string]bool{"auth": true, "config": true}

// TestParityAgainstCobraTree asserts every registry command exists in the
// cobra tree by exact "freshbooks <group> <verb>" path and vice versa.
func TestParityAgainstCobraTree(t *testing.T) {
	root := NewRootCmd("test")

	cobraPaths := make(map[string]bool)
	for _, grp := range root.Commands() {
		if !grp.HasSubCommands() || nonRegistryGroups[grp.Name()] {
			continue
		}
		for _, leaf := range grp.Commands() {
			cobraPaths[grp.Name()+" "+leaf.Name()] = true
		}
	}

	registryPaths := make(map[string]bool, len(All))
	for _, c := range All {
		registryPaths[c.Group+" "+c.Verb] = true
	}

	for p := range registryPaths {
		if !cobraPaths[p] {
			t.Errorf("registry command %q has no cobra leaf", p)
		}
	}
	for p := range cobraPaths {
		if !registryPaths[p] {
			t.Errorf("cobra command %q has no registry entry (or is a non-registry command miscounted here)", p)
		}
	}
}
