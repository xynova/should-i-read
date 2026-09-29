package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/xynova/should-i-read/internal/config"
	"github.com/xynova/should-i-read/internal/token"
)

func newTokenCmd(opts *rootOptions, repoRoot string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "OAuth token helpers for Pimalaya CLIs",
	}
	cmd.AddCommand(newTokenProviderCmd(opts, repoRoot, "gmail", "Gmail OAuth for Neverest XOAUTH2 (print access token)", func(cfg config.Config, account string) token.Provider {
		return token.CreateGmailBroker(cfg, account)
	}))
	cmd.AddCommand(newTokenProviderCmd(opts, repoRoot, "outlook", "Outlook OAuth for Neverest XOAUTH2 (print access token)", func(cfg config.Config, account string) token.Provider {
		return token.CreateOutlookBroker(cfg, account)
	}))
	return cmd
}

func newTokenProviderCmd(
	opts *rootOptions,
	repoRoot string,
	use string,
	short string,
	mkBroker func(config.Config, string) token.Provider,
) *cobra.Command {
	var account string
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
	}
	cmd.PersistentFlags().StringVar(&account, "account", "", "Account label for keyring token namespace")

	cmd.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Browser login; store tokens in OS keyring",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			broker := mkBroker(cfg, account)
			if err := broker.Login(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"ok\":true,\"account\":%q}\n", account)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Remove stored OAuth material from keyring",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			broker := mkBroker(cfg, account)
			if err := broker.Logout(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"ok\":true,\"deleted\":true}\n")
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Redacted client and token storage status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := opts.mustLoad(repoRoot)
			if err != nil {
				return err
			}
			broker := mkBroker(cfg, account)
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(broker.Status())
		},
	})

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		cfg, err := opts.mustLoad(repoRoot)
		if err != nil {
			return err
		}
		broker := mkBroker(cfg, account)
		access, err := broker.AccessToken(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), access)
		return nil
	}
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	return cmd
}
