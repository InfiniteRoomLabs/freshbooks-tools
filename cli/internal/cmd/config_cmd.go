package cmd

import (
	"fmt"

	cliauth "github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/auth"
	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/config"
	"github.com/InfiniteRoomLabs/freshbooks-tools/cli/internal/output"
	"github.com/spf13/cobra"
)

// newConfigCmd builds `config view|contexts|use-context|set-context`
// (D5): config.yaml never carries a secret, only current-context and
// each context's account/business/business_uuid.
//
// --dry-run is rejected here (F3/security B3), matching `auth`: `view`
// and `contexts` are already read-only regardless, but `use-context` and
// `set-context` both write config.yaml, and a flag whose contract is
// "send nothing" silently writing a file is the same surprise the auth
// side closes.
func newConfigCmd(state *runtimeState) *cobra.Command {
	root := &cobra.Command{
		Use:   "config",
		Short: "View and manage config.yaml contexts",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return rejectDryRun(cmd)
		},
	}
	root.AddCommand(newConfigViewCmd(state))
	root.AddCommand(newConfigContextsCmd(state))
	root.AddCommand(newConfigUseContextCmd(state))
	root.AddCommand(newConfigSetContextCmd(state))
	return root
}

func newConfigViewCmd(state *runtimeState) *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Print the resolved config.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := state.loadConfig(cmd)
			if err != nil {
				return err
			}
			return state.writeResult(cmd, cfg)
		},
	}
}

// contextRow is one row of `config contexts`'s table/name output: the
// context name alongside its scope fields, and whether it is the current
// one.
type contextRow struct {
	Name         string `json:"name"`
	Current      bool   `json:"current"`
	Account      string `json:"account,omitempty"`
	Business     string `json:"business,omitempty"`
	BusinessUUID string `json:"business_uuid,omitempty"`
}

func newConfigContextsCmd(state *runtimeState) *cobra.Command {
	return &cobra.Command{
		Use:   "contexts",
		Short: "List the contexts defined in config.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := state.loadConfig(cmd)
			if err != nil {
				return err
			}
			names := output.SortedKeys(cfg.Contexts)
			rows := make([]contextRow, 0, len(names))
			for _, name := range names {
				c := cfg.Contexts[name]
				rows = append(rows, contextRow{
					Name: name, Current: name == cfg.CurrentContext,
					Account: c.Account, Business: c.Business, BusinessUUID: c.BusinessUUID,
				})
			}
			return state.writeResult(cmd, rows)
		},
	}
}

func newConfigUseContextCmd(state *runtimeState) *cobra.Command {
	return &cobra.Command{
		Use:   "use-context <name>",
		Short: "Switch config.yaml's current-context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, path, err := state.loadConfig(cmd)
			if err != nil {
				return err
			}
			if _, ok := cfg.Contexts[name]; !ok {
				return newUsageErrorf("unknown context %q; create it first with 'freshbooks config set-context %s'", name, name)
			}
			cfg.CurrentContext = name
			if err := config.Save(path, cfg); err != nil {
				return &runtimeError{err: err}
			}
			_, err = fmt.Fprintf(state.confirmWriter(cmd), "Switched to context %q.\n", name)
			return err
		},
	}
}

func newConfigSetContextCmd(state *runtimeState) *cobra.Command {
	var account, business, businessUUID string
	cc := &cobra.Command{
		Use:   "set-context <name>",
		Short: "Create or update a context's account/business/business-uuid",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !cliauth.ValidContextName(name) {
				return newUsageErrorf("invalid context name %q: use only letters, digits, '.', '_', '-'", name)
			}
			cfg, path, err := state.loadConfig(cmd)
			if err != nil {
				return err
			}
			if cfg.Contexts == nil {
				cfg.Contexts = map[string]config.Context{}
			}
			c := cfg.Contexts[name]
			if cmd.Flags().Changed("account") {
				c.Account = account
			}
			if cmd.Flags().Changed("business") {
				c.Business = business
			}
			if cmd.Flags().Changed("business-uuid") {
				c.BusinessUUID = businessUUID
			}
			cfg.Contexts[name] = c
			if err := config.Save(path, cfg); err != nil {
				return &runtimeError{err: err}
			}
			_, err = fmt.Fprintf(state.confirmWriter(cmd), "Set context %q.\n", name)
			return err
		},
	}
	cc.Flags().StringVar(&account, "account", "", "the context's FreshBooks account id")
	cc.Flags().StringVar(&business, "business", "", "the context's FreshBooks business id")
	cc.Flags().StringVar(&businessUUID, "business-uuid", "", "the context's FreshBooks business UUID")
	return cc
}
