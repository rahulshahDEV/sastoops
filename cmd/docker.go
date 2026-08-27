package cmd

import (
	"fmt"

	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "ServerOps Docker — install and inspect Docker on servers",
}

func init() {
	dockerCmd.AddCommand(dockerInstallCmd, dockerStatusCmd)
}

var dockerInstallCmd = &cobra.Command{
	Use:   "install [name]",
	Short: "Install Docker Engine + Compose plugin on a server",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		ui.Step("checking docker")
		out, err := client.Output("command -v docker >/dev/null 2>&1 && docker --version || echo missing")
		if err == nil && out != "missing" {
			ui.Ok("docker already installed: %s", out)
			return nil
		}
		ui.Step("installing docker (get.docker.com)")
		if G.Check {
			ui.Info("would install docker on %s", serverName)
			return nil
		}
		out, err = client.Output(`if [ "$(id -u)" -eq 0 ]; then S=''; else S='sudo -n'; fi; $S curl -fsSL https://get.docker.com | $S sh; $S systemctl enable --now docker; docker --version`)
		if err != nil {
			return fmt.Errorf("docker install failed: %s", out)
		}
		ui.Ok("docker installed: %s", lastLine(out))
		return nil
	},
}

var dockerStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show Docker engine + container status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := client.Output(`if command -v docker >/dev/null 2>&1; then docker version --format '{{.Server.Version}}'; else echo missing; fi`)
		if err != nil || out == "missing" {
			ui.Warn("docker is not installed on %s — run: sastoops docker install %s", serverName, serverName)
			return nil
		}
		containers, err := client.Output("docker ps --format 'table {{.Names}}\t{{.Status}}' 2>/dev/null | head -20")
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(map[string]string{"server": serverName, "docker_version": out, "containers": containers})
		}
		ui.KV([][2]string{{"Server", serverName}, {"Docker", out}})
		fmt.Println(containers)
		return nil
	},
}
