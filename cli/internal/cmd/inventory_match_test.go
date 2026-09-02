package cmd

import (
	"encoding/json"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// inventoryEntry is the subset of one freshbooks/internal/inventory/testdata
// /inventory.json record TestRoundTrip's cross-check needs: the vendor
// Postman collection's own method and path template for that key.
type inventoryEntry struct {
	Method       string `json:"method"`
	PathTemplate string `json:"pathTemplate"`
}

// inventoryPath is the vendor inventory relative to this package's test
// working directory (cli/internal/cmd), the same file freshbooks'
// internal/inventory tool normalizes from the Postman collection.
const inventoryPath = "../../../freshbooks/internal/inventory/testdata/inventory.json"

var (
	inventoryOnce  sync.Once
	inventoryIndex map[string]inventoryEntry
	inventoryErr   error
)

// loadInventoryIndex reads inventoryPath once per test binary run, keyed
// by the vendor "key" field (e.g. "Clients/Single Client"), matching
// exactly what Command.Keys entries carry.
func loadInventoryIndex(t *testing.T) map[string]inventoryEntry {
	t.Helper()
	inventoryOnce.Do(func() {
		raw, err := os.ReadFile(inventoryPath)
		if err != nil {
			inventoryErr = err
			return
		}
		var entries []struct {
			Key          string `json:"key"`
			Method       string `json:"method"`
			PathTemplate string `json:"pathTemplate"`
		}
		if err := json.Unmarshal(raw, &entries); err != nil {
			inventoryErr = err
			return
		}
		inventoryIndex = make(map[string]inventoryEntry, len(entries))
		for _, e := range entries {
			inventoryIndex[e.Key] = inventoryEntry{Method: e.Method, PathTemplate: e.PathTemplate}
		}
	})
	if inventoryErr != nil {
		t.Fatalf("loading %s: %v", inventoryPath, inventoryErr)
	}
	return inventoryIndex
}

// inventoryPlaceholderRE matches one placeholder segment in a vendor
// pathTemplate: the vendor collection uses {curlyBraces} for every
// placeholder except one lone holdout using <angle_brackets>
// ("/auth/api/v1/partners/applications/<app_client_id>") -- matching
// both here handles that mechanically instead of special-casing it.
var inventoryPlaceholderRE = regexp.MustCompile(`\{[a-zA-Z]+\}|<[a-zA-Z_]+>`)

// resolveInventoryPath substitutes tmpl's placeholder segments with the
// values a round-trip invocation of c actually used (G4/QA Q6-Q7):
// {accountId}/{businessId}/{businessUuid} are the scope identifiers;
// every other placeholder is a resource id, which for every command in
// the registry except "service-rates update-project-rate" (which has
// two: {projectId} from --project-id and {serviceId}, the positional
// <id>) is the single positional <id> this invocation supplied.
func resolveInventoryPath(c Command, tmpl string) string {
	return inventoryPlaceholderRE.ReplaceAllStringFunc(tmpl, func(ph string) string {
		switch strings.Trim(strings.Trim(ph, "{}"), "<>") {
		case "accountId":
			return string(testScope.AccountID)
		case "businessId":
			return strconv.FormatInt(int64(testScope.BusinessID), 10)
		case "businessUuid":
			return string(testScope.BusinessUUID)
		case "downloadtoken":
			return "probe-download-token"
		case "projectId":
			if c.Group == "service-rates" && c.Verb == "update-project-rate" {
				return "42" // extraFlagArgs' --project-id value, not the positional <id>
			}
		}
		// Every other placeholder ({customerId}, {invoiceId}, {taskId},
		// ...) is the resource id this command's positional <id> names.
		if c.HasID {
			if c.IDKind == "string" {
				return "probe-str-id"
			}
			return "123"
		}
		return ph
	})
}

// inventoryMismatchAllowlist names the one command whose vendor
// pathTemplate cannot be matched mechanically -- not a placeholder-syntax
// quirk, a genuine documented divergence -- rather than forcing a match
// or silently skipping the whole cross-check for it (G4's own
// instruction). The vendor's Postman-captured example for this key is
// tagged family "internal" (every sibling Projects/* entry is "business"),
// and freshbooks/projects.go's ProjectsService.Delete doc comment records
// the decision already made: the captured example disagrees with the
// documented path, and this implementation follows the documented one
// ("the docs win"), unconfirmed live either way.
var inventoryMismatchAllowlist = map[string]string{
	"projects/delete": `vendor key "Projects/Delete Project" pathTemplate is "/comments/business/{businessId}/project/{projectId}" (family "internal", an outlier among sibling Projects/* entries which are all "business"); freshbooks/projects.go's ProjectsService.Delete doc comment records that this implementation deliberately follows the documented path ("/projects/business/.../project/...") instead of the Postman-captured one`,
}

// assertInventoryMatch cross-checks req against the vendor Postman
// collection's own method and path template for c's first inventory key
// (G4/QA Q6-Q7): this ties Command's declarative Service/Method fields
// (and, transitively, what its Run closure actually calls) to an
// independent source, closing the gap where wantPath/wantHost -- captured
// from the implementation's own Run closures -- would freeze a wiring
// error present at capture time as the expectation. identity/whoami is
// the one command with no inventory key and is skipped (nothing to
// check against); inventoryMismatchAllowlist documents the one command
// whose vendor template is a known, deliberate divergence rather than a
// defect.
func assertInventoryMatch(t *testing.T, c Command, req recordedRequest) {
	t.Helper()
	if len(c.Keys) == 0 {
		return
	}
	if reason, skip := inventoryMismatchAllowlist[c.Group+"/"+c.Verb]; skip {
		t.Logf("%s/%s: inventory cross-check skipped -- %s", c.Group, c.Verb, reason)
		return
	}
	idx := loadInventoryIndex(t)
	entry, ok := idx[c.Keys[0]]
	if !ok {
		t.Fatalf("%s/%s: inventory key %q not found in %s", c.Group, c.Verb, c.Keys[0], inventoryPath)
	}
	if req.method != entry.Method {
		t.Errorf("%s/%s: method = %s, want %s (vendor key %q)", c.Group, c.Verb, req.method, entry.Method, c.Keys[0])
	}
	want := resolveInventoryPath(c, entry.PathTemplate)
	if req.path != want {
		t.Errorf("%s/%s: path = %q, want %q (vendor key %q, template %q)", c.Group, c.Verb, req.path, want, c.Keys[0], entry.PathTemplate)
	}
}
