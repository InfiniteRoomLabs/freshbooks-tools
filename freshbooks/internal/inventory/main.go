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

	if err := fs.Parse(args); err != nil {
		return 2
	}

	return runEmit(*in, *out, stdout, stderr)
}

func runEmit(in, out string, stdout, stderr io.Writer) int {
	if in == "" || out == "" {
		fmt.Fprintln(stderr, "inventory: -in and -out are both required")
		return 2
	}

	coll, err := Load(in)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	entries, err := Normalize(coll)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if err := WriteJSON(out, entries); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	fmt.Fprintf(stdout, "inventory: wrote %d entries to %s\n", len(entries), out)
	return 0
}
