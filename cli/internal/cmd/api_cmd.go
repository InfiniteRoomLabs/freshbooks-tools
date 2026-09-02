package cmd

import (
	"encoding/json"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// newAPICmd builds the `freshbooks api` escape hatch (D3): it calls
// (*freshbooks.Client).Do directly with an arbitrary method and path, so
// no lib change is needed to reach an endpoint this CLI does not model
// yet as a registry command.
func newAPICmd(state *runtimeState) *cobra.Command {
	var bodyFile string
	var queryPairs []string

	cc := &cobra.Command{
		Use:   "api <METHOD> <path>",
		Short: "Call an arbitrary FreshBooks API path through the transport's escape hatch",
		Long: "api sends one request through the same authenticated, retrying transport every " +
			"registry command uses, for an endpoint this CLI does not model as its own command " +
			"yet. path is rooted at the API base, e.g. /accounting/account/ACM123/systems/systems/1.",
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			method := strings.ToUpper(args[0])
			path := args[1]

			fullPath, err := appendQuery(path, queryPairs)
			if err != nil {
				return newUsageErrorf("parsing --query: %v", err)
			}

			var bodyVal any
			if bodyFile != "" {
				raw, err := readBodySource(bodyFile, cmd.InOrStdin())
				if err != nil {
					return newUsageErrorf("reading --file: %v", err)
				}
				if len(raw) > 0 {
					// G2/QA Q2: validated the same way and with the same
					// message as the registry path (registry.go), and
					// before buildClient below -- a bad body is a usage
					// error (exit 2) regardless of whether this machine
					// has any credentials, never the lib's own marshal
					// error (which would echo a fragment of the body).
					if !json.Valid(raw) {
						return newUsageError("--file does not contain valid JSON")
					}
					bodyVal = json.RawMessage(raw)
				}
			}

			client, err := state.buildClient(cmd)
			if err != nil {
				return err
			}

			var out json.RawMessage
			if err := client.Do(cmd.Context(), method, fullPath, bodyVal, &out); err != nil {
				if isDryRun(err) {
					return nil
				}
				return classifyRunError(err)
			}
			return state.writeResult(cmd, out)
		},
	}

	cc.Flags().StringVarP(&bodyFile, "file", "f", "", "JSON request body: a file path, or - for stdin")
	// No -q shorthand: the global -q/--quiet persistent flag already
	// claims it, and cobra panics on a duplicate shorthand at merge time.
	cc.Flags().StringArrayVar(&queryPairs, "query", nil, "query parameter as key=value (repeatable)")
	return cc
}

// appendQuery merges key=value pairs into path's existing query string.
func appendQuery(path string, pairs []string) (string, error) {
	if len(pairs) == 0 {
		return path, nil
	}
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return "", newUsageErrorf("invalid --query %q: want key=value", p)
		}
		q.Add(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
