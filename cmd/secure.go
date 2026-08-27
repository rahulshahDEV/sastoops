package cmd

import (
	"fmt"
	"strings"

	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var secureCmd = &cobra.Command{
	Use:     "secure [name]",
	Aliases: []string{"harden"},
	Short:   "Harden a server (SSH, firewall, fail2ban, unattended upgrades)",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		skipFirewall, _ := cmd.Flags().GetBool("skip-firewall")
		adminKey, _ := cmd.Flags().GetString("admin-key")
		overrides := map[string]string{}
		if skipFirewall {
			overrides["allow"] = "22"
		}
		if adminKey != "" {
			overrides["pubkey"] = adminKey
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		ui.Info("hardening %s (idempotent, safe to re-run)", serverName)
		return applyRecipe(client, serverName, "base", overrides)
	},
}

var firewallCmd = &cobra.Command{
	Use:   "firewall [name]",
	Short: "Manage the server firewall (UFW)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		action, _ := cmd.Flags().GetString("action")
		port, _ := cmd.Flags().GetInt("port")
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		switch action {
		case "":
			out, err := client.Output("ufw status verbose")
			if err != nil {
				out2, err2 := client.Output(`if [ "$(id -u)" -eq 0 ]; then S=''; else S='sudo -n'; fi; $S ufw status verbose`)
				if err2 != nil {
					return fmt.Errorf("ufw status: %s", out2)
				}
				out = out2
			}
			if G.JSON {
				return ui.PrintJSON(map[string]string{"status": out})
			}
			fmt.Println(out)
			return nil
		case "allow":
			if port == 0 {
				return fmt.Errorf("--port required with --action allow")
			}
			if !G.Yes && !ui.Confirm(fmt.Sprintf("allow TCP %d on %s?", port, serverName), G.Yes) {
				return fmt.Errorf("aborted")
			}
			out, err := client.Output(fmt.Sprintf(`if [ "$(id -u)" -eq 0 ]; then S=''; else S='sudo -n'; fi; $S ufw allow %d/tcp`, port))
			if err != nil {
				return fmt.Errorf("ufw allow: %s", out)
			}
			ui.Ok("allowed TCP %d", port)
			return nil
		case "deny":
			if port == 0 {
				return fmt.Errorf("--port required with --action deny")
			}
			if !G.Yes && !ui.Confirm(fmt.Sprintf("deny TCP %d on %s?", port, serverName), G.Yes) {
				return fmt.Errorf("aborted")
			}
			out, err := client.Output(fmt.Sprintf(`if [ "$(id -u)" -eq 0 ]; then S=''; else S='sudo -n'; fi; $S ufw deny %d/tcp`, port))
			if err != nil {
				return fmt.Errorf("ufw deny: %s", out)
			}
			ui.Ok("denied TCP %d", port)
			return nil
		}
		return fmt.Errorf("unknown action %q (allow|deny)", action)
	},
}

func init() {
	secureCmd.Flags().Bool("skip-firewall", false, "skip firewall rules")
	secureCmd.Flags().String("admin-key", "", "public key to install for the admin user")
	firewallCmd.Flags().String("action", "", "allow | deny (omit to show status)")
	firewallCmd.Flags().Int("port", 0, "port for allow/deny")
}

var _ = strings.TrimSpace
