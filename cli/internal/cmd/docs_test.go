//go:build docsgen

package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/docsgen"
)

func TestDocsCommand(t *testing.T) {
	t.Run("[happy] writes the generated reference to the given file", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "cli.md")
		code := Run([]string{"docs", out}, discardStdin, io.Discard, io.Discard, "test")
		if code != 0 {
			t.Fatalf("exit = %d", code)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "# freshbooks CLI reference") {
			t.Error("generated file missing the header")
		}
	})

	t.Run("[sad] an unwritable path is a runtime error", func(t *testing.T) {
		code := Run([]string{"docs", "/nonexistent-dir-xyz/cli.md"}, discardStdin, io.Discard, io.Discard, "test")
		if code != 1 {
			t.Fatalf("exit = %d, want 1", code)
		}
	})

	t.Run("[edge] docs is hidden from the generated reference itself", func(t *testing.T) {
		root := NewRootCmd("test")
		content, err := docsgen.Generate(root)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), "## freshbooks docs") {
			t.Error("the hidden docs command should not appear in its own generated reference")
		}
	})
}
