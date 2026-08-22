// Command inventory normalizes the FreshBooks Postman collection into
// inventory.json (emit mode) and checks Go source for the `// inventory:`
// parity contract against that file (check mode).
//
//	go run ./internal/inventory -in <postman.json> -out <inventory.json>
//	go run ./internal/inventory -check ./... [-inventory <inventory.json>] [-ignore <ignore.list>]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inventory", flag.ContinueOnError)
	fs.SetOutput(stderr)

	in := fs.String("in", "", "path to the Postman collection to normalize (emit mode)")
	out := fs.String("out", "", "path to write the normalized inventory.json (emit mode)")
	check := fs.Bool("check", false, "check mode: scan the given packages for // inventory: comments")
	inventoryPath := fs.String("inventory", "internal/inventory/testdata/inventory.json", "inventory.json path (check mode)")
	ignorePath := fs.String("ignore", "internal/inventory/testdata/ignore.list", "ignore/todo list path (check mode)")
	dir := fs.String("dir", "", "working directory to resolve packages from (check mode; default: current directory)")

	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *check {
		return runCheck(fs.Args(), *inventoryPath, *ignorePath, *dir, stdout, stderr)
	}
	return runEmit(*in, *out, stdout, stderr)
}

func runEmit(in, out string, stdout, stderr io.Writer) int {
	if in == "" || out == "" {
		_, _ = fmt.Fprintln(stderr, "inventory: -in and -out are both required unless -check is given")
		return 2
	}

	coll, err := Load(in)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	entries, err := Normalize(coll)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	if err := WriteJSON(out, entries); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "inventory: wrote %d entries to %s\n", len(entries), out)
	return 0
}

func runCheck(pkgs []string, inventoryPath, ignorePath, dir string, stdout, stderr io.Writer) int {
	if len(pkgs) == 0 {
		_, _ = fmt.Fprintln(stderr, "inventory: -check requires at least one package argument, e.g. ./...")
		return 2
	}

	report, err := Check(CheckOptions{
		Packages:      pkgs,
		InventoryPath: inventoryPath,
		IgnorePath:    ignorePath,
		Dir:           dir,
	})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}

	_, _ = fmt.Fprintln(stdout, report.String())
	for _, k := range report.Uncovered {
		_, _ = fmt.Fprintf(stdout, "  uncovered: %s\n", k)
	}
	for _, k := range report.DoubleCovered {
		_, _ = fmt.Fprintf(stdout, "  double-covered: %s\n", k)
	}
	for _, k := range report.Stale {
		_, _ = fmt.Fprintf(stdout, "  stale: %s\n", k)
	}
	for _, k := range report.Unknown {
		_, _ = fmt.Fprintf(stdout, "  unknown: %s\n", k)
	}

	if !report.OK() {
		return 1
	}
	return 0
}
