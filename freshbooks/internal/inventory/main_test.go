package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEmit(t *testing.T) {
	t.Run("[happy] normalizes a real collection", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.json")
		var stdout, stderr bytes.Buffer
		code := run([]string{"-in", filepath.Join("testdata", "freshbooks.postman_collection.json"), "-out", out}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("run() exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "213 entries") {
			t.Errorf("stdout = %q, want it to mention 213 entries", stdout.String())
		}
		if _, err := os.Stat(out); err != nil {
			t.Errorf("output file not written: %v", err)
		}
	})

	t.Run("[sad] missing -in and -out", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run(nil, &stdout, &stderr)
		if code != 2 {
			t.Errorf("run() exit = %d, want 2", code)
		}
	})

	t.Run("[sad] unparseable flags", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"-not-a-flag"}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("run() exit = %d, want 2", code)
		}
	})

	t.Run("[sad] nonexistent input collection", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"-in", "does-not-exist.json", "-out", filepath.Join(t.TempDir(), "out.json")}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("run() exit = %d, want 1", code)
		}
	})

	t.Run("[sad] unwritable output path", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{
			"-in", filepath.Join("testdata", "freshbooks.postman_collection.json"),
			"-out", filepath.Join(t.TempDir(), "nonexistent-dir", "out.json"),
		}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("run() exit = %d, want 1", code)
		}
	})
}

func TestRunCheck(t *testing.T) {
	t.Run("[happy] passing check exits 0", func(t *testing.T) {
		dir := newFixtureModule(t, map[string]string{
			"impl.go": "package fixture\n\n// inventory: Clients/List Clients\nfunc A() {}\n",
		})
		inv := writeInventoryFixture(t, "Clients/List Clients")
		ignore := writeIgnoreFixture(t)

		var stdout, stderr bytes.Buffer
		code := run([]string{"-check", "-inventory", inv, "-ignore", ignore, "-dir", dir, "./..."}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("run() exit = %d, stderr = %s", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "implemented 1") {
			t.Errorf("stdout = %q", stdout.String())
		}
	})

	t.Run("[sad] failing check exits 1 and lists findings", func(t *testing.T) {
		inv := writeInventoryFixture(t, "Clients/List Clients")
		ignore := writeIgnoreFixture(t)

		var stdout, stderr bytes.Buffer
		code := run([]string{"-check", "-inventory", inv, "-ignore", ignore, "./..."}, &stdout, &stderr)
		if code != 1 {
			t.Fatalf("run() exit = %d, want 1", code)
		}
		if !strings.Contains(stdout.String(), "uncovered: Clients/List Clients") {
			t.Errorf("stdout = %q, want an uncovered line", stdout.String())
		}
	})

	t.Run("[sad] no package arguments", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"-check"}, &stdout, &stderr)
		if code != 2 {
			t.Errorf("run() exit = %d, want 2", code)
		}
	})

	t.Run("[sad] Check error (bad inventory path) surfaces as exit 1", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		code := run([]string{"-check", "-inventory", "does-not-exist.json", "./..."}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("run() exit = %d, want 1", code)
		}
	})
}
