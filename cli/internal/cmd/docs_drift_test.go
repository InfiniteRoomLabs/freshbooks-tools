package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docsMDPath is docs/cli.md relative to this package.
const docsMDPath = "../../../docs/cli.md"

// TestDocsUpToDate regenerates the full command reference (through the
// same `docs` command scripts/docs.sh invokes, not GenerateDocs called on
// a bare, unexecuted root -- cobra only adds the default `completion`
// subcommand as a side effect of Execute(), so a root that never went
// through Run/Execute would omit it and always "drift") and fails if it
// does not byte-for-byte match the committed docs/cli.md, naming the
// exact command to fix it.
func TestDocsUpToDate(t *testing.T) {
	committed, err := os.ReadFile(docsMDPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docsMDPath, err)
	}

	out := filepath.Join(t.TempDir(), "cli.md")
	var stdout, stderr strings.Builder
	if code := Run([]string{"docs", out}, discardStdin, &stdout, &stderr, "test"); code != 0 {
		t.Fatalf("Run(docs) exit = %d, stderr = %s", code, stderr.String())
	}
	generated, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reading the regenerated docs: %v", err)
	}

	if string(generated) != string(committed) {
		t.Fatalf("%s is out of date with the current command tree; run `mise run docs` (scripts/docs.sh) and commit the result", docsMDPath)
	}
}
