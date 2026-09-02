package cmd

import (
	"errors"
	"fmt"

	"github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks"
	fbauth "github.com/InfiniteRoomLabs/freshbooks-tools/freshbooks/auth"
)

// exitCoder is implemented by every error type this package constructs
// directly, so exitCodeFor can report the precise D6 exit code without
// re-deriving it from string matching.
type exitCoder interface {
	ExitCode() int
}

// usageError is a CLI-level usage problem: a cobra parse error, a bad JSON
// body, a missing scope, --all combined with --page/--per-page, an
// unknown --output value, or a destructive command run without --yes.
// Exit code 2.
type usageError struct{ msg string }

func newUsageError(msg string) *usageError { return &usageError{msg: msg} }

func newUsageErrorf(format string, args ...any) *usageError {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

func (e *usageError) Error() string { return e.msg }
func (e *usageError) ExitCode() int { return 2 }

// authError is "no credentials for the context" or anything else in the
// auth path that is not itself an API response. An API 401 is handled
// separately by exitCodeFor inspecting *freshbooks.Error directly -- it
// never needs to be wrapped in authError. Exit code 3.
type authError struct{ msg string }

func newAuthErrorf(format string, args ...any) *authError {
	return &authError{msg: fmt.Sprintf(format, args...)}
}

func (e *authError) Error() string { return e.msg }
func (e *authError) ExitCode() int { return 3 }

// runtimeError wraps any error this package's own plumbing produces that
// is not a usage, auth, or API problem: a corrupt (unparseable)
// credentials or config.yaml file, a docs-generation failure, building
// the *freshbooks.Client itself, or writing a Binary command's result to
// a local file. Exit code 1. (Q16/G6: this comment used to say "a
// filesystem failure reading --file" -- every one of those is actually a
// usageError, exit 2: registry.go's body read, api_cmd.go's, and
// invocation.go's OpenUpload all reject a bad --file as the caller's
// mistake, not a runtime failure.) A *freshbooks.Error is never wrapped
// in this: exitCodeFor inspects it directly so its status code
// (401/404/other) drives the exit code.
type runtimeError struct{ err error }

func (e *runtimeError) Error() string { return e.err.Error() }
func (e *runtimeError) Unwrap() error { return e.err }
func (e *runtimeError) ExitCode() int { return 1 }

// classifyRunError normalizes an error a Command.Run closure returned: an
// already-typed error (usageError/authError/*freshbooks.Error, which all
// carry their own exit code one way or another) passes through unchanged;
// anything else is wrapped as a runtimeError so exitCodeFor never
// mistakes it for a bare cobra parsing error (see exitCodeFor's doc
// comment for why that distinction matters).
func classifyRunError(err error) error {
	if err == nil {
		return nil
	}
	var ec exitCoder
	if errors.As(err, &ec) {
		return err
	}
	var fbErr *freshbooks.Error
	if errors.As(err, &fbErr) {
		return err
	}
	var authErr *fbauth.Error
	if errors.As(err, &authErr) {
		return err
	}
	return &runtimeError{err: err}
}

// exitCodeFor maps an error returned by cobra's root.Execute() to a D6
// exit code:
//
//   - nil                                     -> 0
//   - a type implementing exitCoder            -> that code (2 or 3)
//   - a *freshbooks.Error with status 401       -> 3
//   - a *freshbooks.Error with status 404       -> 4
//   - any other *freshbooks.Error                -> 1
//   - a *freshbooks/auth.Error (an OAuth token
//     endpoint error, from `auth token --refresh`
//     or `auth login`) with status 401           -> 3
//   - any other *freshbooks/auth.Error, or a
//     runtimeError-wrapped error                -> 1
//   - anything else (untyped)                   -> 2
//
// The last case is deliberate, not a catch-all default: every error this
// package's own command execution path returns is wrapped by
// classifyRunError into one of the typed cases above before it ever
// reaches Execute(), so an untyped error reaching here can only have come
// from cobra itself -- an unknown command or a positional-argument count
// or flag cobra rejected before any RunE ran (this package registers no
// FlagErrorFunc; cobra's own default flag-parse error is exactly such an
// untyped error, so it falls through to exit 2 here rather than being
// intercepted earlier). All of those are usage problems.
func exitCodeFor(err error) int {
	if err == nil {
		return 0
	}
	var ec exitCoder
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	var fbErr *freshbooks.Error
	if errors.As(err, &fbErr) {
		switch fbErr.StatusCode {
		case 401:
			return 3
		case 404:
			return 4
		default:
			return 1
		}
	}
	var authErr *fbauth.Error
	if errors.As(err, &authErr) {
		if authErr.StatusCode == 401 {
			return 3
		}
		return 1
	}
	return 2
}
