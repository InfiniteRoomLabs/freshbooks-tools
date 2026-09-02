package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Class is a registry command's annotation class, matching the RO/D/I/W
// columns in docs/phases/4/commands.md and mcp/internal/tools/registry.go.
type Class string

// The four annotation classes docs/phases/4/commands.md documents.
const (
	ClassRO Class = "RO" // read-only
	ClassD  Class = "D"  // destructive
	ClassI  Class = "I"  // idempotent
	ClassW  Class = "W"  // a plain write, neither destructive nor idempotent
)

// ScopeFamily says which scope flag(s) a command's lib method needs. The
// family comes from the lib method's own signature, never converted
// (CLAUDE.md's "two ID families" gotcha) -- ScopeAccountAndBusiness exists
// for the one method (SystemsService.Get) that genuinely needs both at
// once, not as a shortcut for anything else.
type ScopeFamily int

// The scope families a Command declares.
const (
	ScopeNone ScopeFamily = iota
	ScopeAccount
	ScopeBusiness
	ScopeBusinessUUID
	ScopeAccountAndBusiness
)

// Scope carries the resolved account/business identifiers a Run closure
// calls the lib with.
type Scope struct {
	AccountID    freshbooks.AccountID
	BusinessID   freshbooks.BusinessID
	BusinessUUID freshbooks.BusinessUUID
}

// RunFunc is one command's implementation: given the resolved client and
// invocation, it calls exactly one freshbooks client-library method and
// returns its result (or a []byte for a Binary command). RunFunc
// implementations never format or print the result themselves and never
// build error messages by interpolating a raw request body -- registry.go
// and invocation.go's generic plumbing handle output formatting, exit
// codes, and redaction.
type RunFunc func(ctx context.Context, c *freshbooks.Client, inv *Invocation) (any, error)

// Command is one registry entry: everything BuildTree needs to generate a
// cobra command, and everything the parity test needs to check it against
// the lib and docs/phases/4/commands.md.
type Command struct {
	// Group is the cobra parent command name, e.g. "clients". Verb is the
	// leaf command name, e.g. "list". Together they are the
	// "freshbooks <group> <verb>" path docs/phases/4/commands.md lists.
	Group, Verb string
	// Short is the one-line help text.
	Short string
	// Service and Method are the *freshbooks.Client field name and its
	// exported method this command wraps, e.g. "Clients", "List".
	Service, Method string
	// Keys are the lib method's // inventory: keys, stacked exactly as
	// freshbooks carries them.
	Keys []string
	// Class is the RO/D/I/W annotation.
	Class Class
	// Scope says which scope flag(s) this command resolves.
	Scope ScopeFamily

	// HasID marks a positional <id> argument. IDKind is "int64" (the
	// default, parsed and validated) or "string" (passed through as-is).
	// IDName is the positional's display name in Use and error messages;
	// "id" when empty.
	HasID  bool
	IDKind string
	IDName string

	// ExtraPositional names positional arguments beyond <id>, in order,
	// e.g. {"query"} for time-entries search or {"download-token"} for
	// reports download-invoice-details-csv.
	ExtraPositional []string

	// List registers --page/--per-page (unless NoPaging) and --search.
	// HasInclude additionally registers --include; HasAll additionally
	// registers --all (rejecting --page/--per-page alongside it).
	// NoPaging is set for the handful of List methods whose lib options
	// struct carries no Page/PerPage field at all (only Search) --
	// registering the flags would silently do nothing.
	List       bool
	HasInclude bool
	HasAll     bool
	NoPaging   bool
	// HasSort registers --sort field[:asc|desc] on a List command whose
	// lib method takes extra ...RequestOption (spec section 7's command
	// surface names this flag explicitly, not as a heuristic -- 21 of the
	// 24 List methods qualify; the other three take no options at all).
	// Invocation.SortOpt() renders it as freshbooks.Sort(field, dir),
	// appended to the same extra variadic every List/All call already
	// takes.
	HasSort bool

	// Body registers -f/--file for a JSON request body (a path, or - for
	// stdin), decoded by the Run closure via Invocation.DecodeBody.
	// BodyOptional (the 13 report commands with a filter-options struct
	// FreshBooks itself defaults sensibly) lets the command run with no
	// --file at all; the Run closure reads it back via
	// Invocation.DecodeBodyOptional instead.
	Body         bool
	BodyOptional bool
	// Upload registers --file for a local file to upload, opened by the
	// Run closure via Invocation.OpenUpload.
	Upload bool
	// Binary marks a command whose result is raw bytes, written to
	// -o/--output (a local -o that shadows the global output-format
	// flag on this one command, since a binary result has no format to
	// select) instead of going through the normal formatter.
	Binary bool

	// ExtraFlags registers any command-specific flags beyond the above
	// (--verifier, --message, --rate, --project-id, --payment-method),
	// read back by the Run closure via Invocation.Flags.
	ExtraFlags func(fs *pflag.FlagSet)
	// RequiredFlags names ExtraFlags-registered string flags that must be
	// non-empty, checked in execute() before buildClient (F13/review A1):
	// a missing --verifier or --message is a usage error (exit 2)
	// regardless of whether the machine has any stored credentials at
	// all, not an auth error discovered only after a client was built for
	// a call that was never going to happen. Each Run closure still reads
	// the flag itself (via Invocation.Flags or a RequiredString helper);
	// this list only decides how early the "is it there" check runs.
	// RequiredInt64Flags is the same idea for an ExtraFlags-registered
	// int64 flag that must be non-zero (service-rates update-project-rate's
	// --project-id).
	RequiredFlags      []string
	RequiredInt64Flags []string

	// Run is the one-line closure that calls the wrapped lib method.
	Run RunFunc
}

// use renders the command's cobra Use line: verb plus every positional
// argument in order.
func (c Command) use() string {
	parts := []string{c.Verb}
	if c.HasID {
		name := c.IDName
		if name == "" {
			name = "id"
		}
		parts = append(parts, "<"+name+">")
	}
	for _, p := range c.ExtraPositional {
		parts = append(parts, "<"+p+">")
	}
	return strings.Join(parts, " ")
}

// wantArgs is how many positional arguments this command expects.
func (c Command) wantArgs() int {
	n := len(c.ExtraPositional)
	if c.HasID {
		n++
	}
	return n
}

// argsValidator returns a cobra.PositionalArgs that rejects the wrong
// argument count with a usageError (exit 2) instead of cobra's default
// message, so exitCodeFor's typed-error path handles it uniformly with
// every other usage problem.
func (c Command) argsValidator() cobra.PositionalArgs {
	want := c.wantArgs()
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != want {
			return newUsageErrorf("%s: expected %d positional argument(s), got %d", cmd.CommandPath(), want, len(args))
		}
		return nil
	}
}

// registerFlags adds every flag this command's shape implies.
func (c Command) registerFlags(cc *cobra.Command) {
	if c.List {
		if !c.NoPaging {
			cc.Flags().Int("page", 0, "1-based page number")
			cc.Flags().Int("per-page", 0, "page size")
		}
		cc.Flags().StringToString("search", nil, "filter as key=value (repeatable)")
		if c.HasAll {
			cc.Flags().Bool("all", false, "walk every page and return every item (rejects --page/--per-page)")
		}
		if c.HasSort {
			cc.Flags().String("sort", "", "sort as field[:asc|desc] (default asc; direction encoding for business-scoped resources is unconfirmed against the live API -- see docs/progress.md)")
		}
	}
	// HasInclude is decoupled from List (F9/review B7): several
	// non-List commands (invoices get/create/update, invoice-profiles
	// get/create, and the other single-resource get commands whose lib
	// method takes opts ...RequestOption) also accept --include.
	if c.HasInclude {
		cc.Flags().StringArray("include", nil, "related sub-resource to embed in the response (repeatable)")
	}
	if c.Body {
		help := "JSON request body: a file path, or - for stdin (required)"
		if c.BodyOptional {
			help = "JSON filter options: a file path, or - for stdin (optional)"
		}
		cc.Flags().StringP("file", "f", "", help)
	}
	if c.Upload {
		cc.Flags().String("file", "", "path to the local file to upload (required)")
	}
	if c.Binary {
		// This -o/--output shadows the global -o/--output format flag on
		// this one command (F18/review A8): a binary result has no
		// format to select, so the local flag takes a file path instead.
		cc.Flags().StringP("output", "o", "-", "write the result to this file, or - for stdout (shadows the global -o/--output format flag on this command); refuses to overwrite an existing file without --force, and refuses - on a TTY")
		cc.Flags().Bool("force", false, "overwrite an existing -o/--output file")
	}
	if c.ExtraFlags != nil {
		c.ExtraFlags(cc.Flags())
	}
}

// destructiveSuffix is appended to a ClassD command's Short so a reader
// of `docs/cli.md` (or --help) can actually tell which commands need
// --yes, instead of every command inheriting the identical --yes flag
// help line regardless of class (F5/review B3).
const destructiveSuffix = " (destructive: requires --yes on a TTY)"

// buildCobra builds this command's leaf *cobra.Command, bound to state.
func (c Command) buildCobra(state *runtimeState) *cobra.Command {
	short := c.Short
	if c.Class == ClassD {
		short += destructiveSuffix
	}
	cc := &cobra.Command{
		Use:   c.use(),
		Short: short,
		Args:  c.argsValidator(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.execute(cmd, args, state)
		},
	}
	c.registerFlags(cc)
	return cc
}

// execute is this command's RunE body: resolve scope, parse positionals,
// gate destructive commands on --yes, parse list/body/upload flags,
// build a client (dry-run aware), call Run, and format the result.
func (c Command) execute(cmd *cobra.Command, args []string, state *runtimeState) error {
	scope, err := state.resolveScope(cmd, c.Scope)
	if err != nil {
		return err
	}

	inv := &Invocation{Scope: scope, Flags: cmd.Flags()}

	idx := 0
	if c.HasID {
		raw := args[idx]
		idx++
		if c.IDKind == "string" {
			inv.idStr = raw
		} else {
			n, perr := strconv.ParseInt(raw, 10, 64)
			if perr != nil {
				name := c.IDName
				if name == "" {
					name = "id"
				}
				return newUsageErrorf("invalid %s %q: must be an integer", name, raw)
			}
			inv.idInt = n
		}
	}
	inv.extra = args[idx:]

	if c.Class == ClassD {
		yes, _ := cmd.Flags().GetBool("yes")
		if !yes && stdinIsTerminal(cmd.InOrStdin()) {
			return newUsageError("this is a destructive operation; re-run with --yes to confirm")
		}
	}

	if c.List {
		if !c.NoPaging {
			inv.page, _ = cmd.Flags().GetInt("page")
			inv.perPage, _ = cmd.Flags().GetInt("per-page")
		}
		inv.search, _ = cmd.Flags().GetStringToString("search")
		if c.HasAll {
			inv.all, _ = cmd.Flags().GetBool("all")
			if inv.all && (inv.page != 0 || inv.perPage != 0) {
				return newUsageError("--all cannot be combined with --page or --per-page")
			}
		}
		if c.HasSort {
			raw, _ := cmd.Flags().GetString("sort")
			field, dir, err := parseSort(raw)
			if err != nil {
				return err
			}
			inv.sortField, inv.sortDir = field, dir
		}
	}
	if c.HasInclude {
		inv.include, _ = cmd.Flags().GetStringArray("include")
	}

	if c.Body {
		path, _ := cmd.Flags().GetString("file")
		if path == "" {
			if !c.BodyOptional {
				return newUsageError("--file is required (a JSON file path, or - for stdin)")
			}
		} else {
			raw, err := readBodySource(path, cmd.InOrStdin())
			if err != nil {
				return newUsageErrorf("reading --file: %v", err)
			}
			// Validated here, not left to the Run closure's later
			// DecodeBody call: a malformed body is a usage error (exit 2)
			// regardless of whether this machine has any credentials at
			// all, and DecodeBody only runs after buildClient below
			// (F13/review A1).
			if !json.Valid(raw) {
				return newUsageError("--file does not contain valid JSON")
			}
			inv.body, inv.hasBody = raw, true
		}
	}

	if c.Upload {
		inv.uploadPath, _ = cmd.Flags().GetString("file")
		if inv.uploadPath == "" {
			return newUsageError("--file is required")
		}
	}

	for _, name := range c.RequiredFlags {
		v, _ := cmd.Flags().GetString(name)
		if v == "" {
			return newUsageErrorf("--%s is required", name)
		}
	}
	for _, name := range c.RequiredInt64Flags {
		v, _ := cmd.Flags().GetInt64(name)
		if v == 0 {
			return newUsageErrorf("--%s is required", name)
		}
	}

	client, err := state.buildClient(cmd)
	if err != nil {
		return err
	}

	result, runErr := c.Run(cmd.Context(), client, inv)
	if runErr != nil {
		if isDryRun(runErr) {
			return nil
		}
		return classifyRunError(runErr)
	}

	if c.Binary {
		path, _ := cmd.Flags().GetString("output")
		force, _ := cmd.Flags().GetBool("force")
		return writeBinaryResult(cmd, result, path, force)
	}
	return state.writeResult(cmd, result)
}

// All is the full command registry: one Command per freshbooks
// client-library method the phase 4 work order covers (168 entries,
// docs/phases/4/commands.md). Declared as a function so it evaluates
// after every commandsXxx slice below has initialized, regardless of Go's
// package-level var initialization order across files.
var All = buildRegistry()

func buildRegistry() []Command {
	return sortedCommands(joinAll(
		attachmentsCommands, billPaymentsCommands, billsCommands, billVendorsCommands,
		callbacksCommands, clientsCommands, contactsCommands, creditNotesCommands,
		estimatesCommands, expenseCategoriesCommands, expensesCommands, gatewaysCommands,
		identityCommands, imagesCommands, invoiceProfilesCommands, invoicesCommands,
		itemsCommands, journalCommands, ledgerAccountsCommands, otherIncomeCommands,
		paymentOptionsCommands, paymentsCommands, projectsCommands, reportsCommands,
		retainersCommands, serviceRatesCommands, servicesCommands, staffCommands,
		systemsCommands, tasksCommands, taxesCommands, teamMembersCommands, timeEntriesCommands,
	))
}

func joinAll(groups ...[]Command) []Command {
	var out []Command
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// sortedCommands orders commands by group then verb, so BuildTree's cobra
// tree and the generated docs are deterministic regardless of the order
// the per-resource files declared their slices in.
func sortedCommands(cmds []Command) []Command {
	sort.SliceStable(cmds, func(i, j int) bool {
		if cmds[i].Group != cmds[j].Group {
			return cmds[i].Group < cmds[j].Group
		}
		return cmds[i].Verb < cmds[j].Verb
	})
	return cmds
}

// BuildTree adds every registry command to root, grouped under one cobra
// parent command per resource (e.g. "clients"), bound to state.
func BuildTree(root *cobra.Command, state *runtimeState) {
	groups := make(map[string]*cobra.Command)
	for _, c := range All {
		grp, ok := groups[c.Group]
		if !ok {
			grp = &cobra.Command{Use: c.Group, Short: fmt.Sprintf("Manage %s", c.Group)}
			groups[c.Group] = grp
			root.AddCommand(grp)
		}
		grp.AddCommand(c.buildCobra(state))
	}
}

// collectAll drains a lib All iterator into a slice, stopping at the
// first error -- the shared implementation every --all-capable List
// command's Run closure calls into.
func collectAll[T any](seq func(yield func(T, error) bool)) ([]T, error) {
	var items []T
	for item, err := range seq {
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// void adapts a lib method that returns only an error into a RunFunc's
// (any, error) shape: nil passes through as a small acknowledgement, so a
// successful call still carries content instead of an empty result.
func void(err error) (any, error) {
	if err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}
