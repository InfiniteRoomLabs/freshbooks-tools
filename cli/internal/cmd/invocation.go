package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	"github.com/spf13/pflag"
)

// Invocation carries one command invocation's parsed request context: the
// resolved scope, positional arguments, list/body/upload flag values, and
// the command's own FlagSet for any command-specific flag a Run closure
// needs (--verifier, --message, --rate, ...). registry.go's Command.execute
// populates it before calling Run; Run closures never parse cobra flags
// directly except through the accessors here or via Flags for a
// command-specific one.
type Invocation struct {
	// Scope carries the resolved account/business/business-uuid
	// identifiers for this call.
	Scope Scope

	// Flags is this command's own FlagSet, for ExtraFlags-registered
	// flags a Run closure reads directly (e.g. cmd.Flags().GetString
	// via inv.Flags.GetString("verifier")).
	Flags *pflag.FlagSet

	idInt int64
	idStr string

	extra []string

	body    []byte
	hasBody bool

	page, perPage int
	search        map[string]string
	include       []string
	all           bool

	uploadPath string
}

// IntID returns the positional <id> argument as an int64. Valid only for
// a command with HasID and IDKind != "string"; registry.go's
// Command.execute has already validated and parsed it.
func (inv *Invocation) IntID() int64 { return inv.idInt }

// StrID returns the positional <id> argument as-is. Valid only for a
// command with HasID and IDKind == "string".
func (inv *Invocation) StrID() string { return inv.idStr }

// Extra returns the i'th ExtraPositional argument (0-based, after <id>
// when present), or "" if there is none at that index.
func (inv *Invocation) Extra(i int) string {
	if i < 0 || i >= len(inv.extra) {
		return ""
	}
	return inv.extra[i]
}

// DecodeBody decodes the -f/--file JSON body into out, rejecting unknown
// fields. Returns a usageError (exit 2) if no body was supplied or the
// JSON is malformed or carries a field out does not have -- both are the
// caller's mistake, never the API's.
func (inv *Invocation) DecodeBody(out any) error {
	if !inv.hasBody {
		return newUsageError("this command requires --file/-f to supply a JSON body")
	}
	dec := json.NewDecoder(bytes.NewReader(inv.body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return newUsageErrorf("decoding the request body: %v", err)
	}
	return nil
}

// DecodeBodyOptional is DecodeBody for a command whose -f/--file body is
// optional (the 13 report commands that take a filter-options struct
// FreshBooks itself defaults sensibly when omitted): it reports whether a
// body was supplied, decoding into out only when one was.
func (inv *Invocation) DecodeBodyOptional(out any) (bool, error) {
	if !inv.hasBody {
		return false, nil
	}
	if err := inv.DecodeBody(out); err != nil {
		return false, err
	}
	return true, nil
}

// Search returns the --search filter as the lib's Search type, nil when
// no --search flag was given (so a Call never sends an empty search[]
// query parameter).
func (inv *Invocation) Search() freshbooks.Search {
	if len(inv.search) == 0 {
		return nil
	}
	return freshbooks.Search(inv.search)
}

// Page returns the --page value (0 when unset, which every
// ListOptions.opts() in the lib already treats as "omit").
func (inv *Invocation) Page() int { return inv.page }

// PerPage returns the --per-page value (0 when unset, same as Page).
func (inv *Invocation) PerPage() int { return inv.perPage }

// Include returns the --include values, in the order given.
func (inv *Invocation) Include() []string { return inv.include }

// All reports whether --all was given.
func (inv *Invocation) All() bool { return inv.all }

// ReqOpts renders Search/Page/PerPage as a []RequestOption, for the
// handful of lib methods that take raw variadic RequestOptions instead of
// a *XListOptions struct (JournalEntries.Details,
// JournalEntryAccounts.List).
func (inv *Invocation) ReqOpts() []freshbooks.RequestOption {
	var opts []freshbooks.RequestOption
	if s := inv.Search(); len(s) > 0 {
		opts = append(opts, s)
	}
	if inv.page > 0 {
		opts = append(opts, freshbooks.PageNumber(inv.page))
	}
	if inv.perPage > 0 {
		opts = append(opts, freshbooks.PerPage(inv.perPage))
	}
	return opts
}

// OpenUpload opens the --file path for an Upload command, returning the
// open file and its base name (directory components stripped, matching
// what the lib's own multipart builder does defensively on its side too).
// The caller must close the returned reader.
func (inv *Invocation) OpenUpload() (io.ReadCloser, string, error) {
	f, err := os.Open(inv.uploadPath) // #nosec G304 -- an operator-supplied CLI flag, not a request
	if err != nil {
		return nil, "", newUsageErrorf("opening --file %q: %v", inv.uploadPath, err)
	}
	return f, filepath.Base(inv.uploadPath), nil
}

// readBodySource reads a -f/--file value: "-" reads stdin, anything else
// is a local file path.
func readBodySource(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path) // #nosec G304 -- an operator-supplied CLI flag, not a request
}
