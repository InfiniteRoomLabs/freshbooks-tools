package cmd

import (
	"os"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/docsgen"
)

// docsMDPath is docs/cli.md relative to this package.
const docsMDPath = "../../../docs/cli.md"

// TestDocsUpToDate regenerates the full command reference by calling
// docsgen.Generate directly (not through the docsgen-tagged `docs`
// command -- this test stays untagged so it keeps running in the default
// gate) and fails if it does not byte-for-byte match the committed
// docs/cli.md, naming the exact command to fix it.
//
// cobra only adds the default `completion` subcommand as a side effect of
// ExecuteC() (via InitDefaultCompletionCmd); a root that never went
// through Run/Execute would omit it and always "drift" against what
// `mise run docs` (which does go through the docs command's RunE, itself
// reached via Run -> rootCmd.Execute()) actually produces. Calling
// InitDefaultCompletionCmd directly reproduces that one side effect
// without needing to actually execute the tree.
func TestDocsUpToDate(t *testing.T) {
	committed, err := os.ReadFile(docsMDPath)
	if err != nil {
		t.Fatalf("reading %s: %v", docsMDPath, err)
	}

	root := NewRootCmd("test")
	root.InitDefaultCompletionCmd()
	generated, err := docsgen.Generate(root)
	if err != nil {
		t.Fatalf("docsgen.Generate: %v", err)
	}

	if string(generated) != string(committed) {
		t.Fatalf("%s is out of date with the current command tree; run `mise run docs` (scripts/docs.sh) and commit the result", docsMDPath)
	}
}
