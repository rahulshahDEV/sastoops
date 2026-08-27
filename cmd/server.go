package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"github.com/rahulshahDEV/sastoops/internal/provider"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:     "server",
	Aliases: []string{"srv"},
	Short:   "ServerOps Cloud — manage servers (provision, import, SSH, run)",
}

func init() {
	serverCmd.AddCommand(
		serverAddCmd, serverCreateCmd, serverListCmd, serverInfoCmd,
		serverSSHCmd, serverRunCmd, serverStatusCmd, serverRebootCmd, serverDeleteCmd,
	)
}

var serverAddCmd = &cobra.Command{
	Use:   "add <name> <user>@<host>",
	Short: "Import an existing server (generic VPS) into ServerOps",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		user, host, err := parseUserHost(args[1])
		if err != nil {
			return err
		}
		port, _ := cmd.Flags().GetInt("port")
		key, _ := cmd.Flags().GetString("key")
		region, _ := cmd.Flags().GetString("region")

		c, err := config.LoadOrNew(G.ConfigPath)
		if err != nil {
			return err
		}
		if _, exists := c.Servers[name]; exists {
			return fmt.Errorf("server %q already exists", name)
		}
		c.Servers[name] = &config.Server{
			Host:     host,
			Port:     port,
			User:     user,
			KeyPath:  key,
			Region:   region,
			Provider: "generic",
		}
		if err := c.Save(G.ConfigPath); err != nil {
			return err
		}
		ui.Ok("added server %s (%s@%s)", name, user, host)

		if auto, _ := cmd.Flags().GetBool("test"); auto {
			client, _, err := dial(name)
			if err != nil {
				ui.Warn("could not connect (edit ~/.config/sastoops/config.yaml): %v", err)
				return nil
			}
			client.Close()
			ui.Ok("SSH connection verified")
		}
		return nil
	},
}

var serverCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Provision a new server via a cloud provider (hetzner, …)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		providerName, _ := cmd.Flags().GetString("provider")
		typ, _ := cmd.Flags().GetString("type")
		region, _ := cmd.Flags().GetString("region")
		image, _ := cmd.Flags().GetString("image")
		sshKey, _ := cmd.Flags().GetString("ssh-key")
		autoSetup, _ := cmd.Flags().GetBool("setup")

		c, err := config.LoadOrNew(G.ConfigPath)
		if err != nil {
			return err
		}
		if _, exists := c.Servers[name]; exists {
			return fmt.Errorf("server %q already exists", name)
		}
		reg := provider.NewRegistry(c)
		p, err := reg.Get(providerName)
		if err != nil {
			return err
		}
		if G.Check {
			ui.Step("would create %s on %s (type=%s region=%s image=%s)", name, providerName, typ, region, image)
			return nil
		}
		ui.Step("provisioning %s on %s", name, providerName)
		ctx := context.Background()
		machine, err := p.Create(ctx, provider.CreateRequest{
			Name: name, Type: typ, Region: region, Image: image, SSHKeyName: sshKey,
		})
		if err != nil {
			return err
		}
		ui.Ok("created: %s (%s)", machine.ID, machine.Status)
		if machine.IP == "" {
			ui.Info("waiting for IP address…")
			for i := 0; i < 30; i++ {
				time.Sleep(5 * time.Second)
				if m, err := p.Get(ctx, machine.ID); err == nil && m.IP != "" {
					machine.IP = m.IP
					break
				}
			}
		}
		ui.Ok("IP: %s", machine.IP)
		c.Servers[name] = &config.Server{
			Host:       machine.IP,
			User:       "root",
			Provider:   providerName,
			ProviderID: machine.ID,
			Region:     machine.Region,
			Type:       machine.Type,
		}
		if err := c.Save(G.ConfigPath); err != nil {
			return err
		}
		ui.Ok("saved server %s", name)
		if autoSetup {
			ui.Step("waiting for SSH…")
			waitForSSH(machine.IP, 60)
			ui.Step("running setup: sastoops server setup %s", name)
			return runServerSetup(name, "base")
		}
		ui.Info("next: sastoops server setup %s", name)
		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List configured servers",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getConfig()
		if err != nil {
			return err
		}
		if len(c.Servers) == 0 {
			ui.Info("no servers configured — run: sastoops server add <name> <user>@<host>")
			return nil
		}
		if G.JSON {
			type row struct {
				Name, Host, User, Provider, Region string
				Port                               int
			}
			rows := []row{}
			for _, n := range ui.SortedKeys(c.Servers) {
				s := c.Servers[n]
				rows = append(rows, row{n, s.Host, s.User, s.Provider, s.Region, s.Port})
			}
			return ui.PrintJSON(rows)
		}
		t := ui.NewTable("NAME", "HOST", "PORT", "USER", "PROVIDER", "REGION")
		for _, n := range ui.SortedKeys(c.Servers) {
			s := c.Servers[n]
			port := "22"
			if s.Port != 0 {
				port = fmt.Sprintf("%d", s.Port)
			}
			p := s.Provider
			if p == "" {
				p = "generic"
			}
			t.Add(n, s.Host, port, s.User, p, s.Region)
		}
		t.Render()
		return nil
	},
}

var serverInfoCmd = &cobra.Command{
	Use:     "info [name]",
	Aliases: []string{"inspect"},
	Short:   "Show OS, uptime, load, disk and memory for a server",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := resolveServerName(oneArg(args))
		client, _, err := dial(name)
		if err != nil {
			return err
		}
		defer client.Close()
		ui.Info("connected to %s", name)
		script := `printf '%s\n' \
	"$(. /etc/os-release 2>/dev/null && echo "$PRETTY_NAME" || echo unknown)" \
	"$(uptime -p)" \
	"$(cat /proc/loadavg)" \
	"$(df -h / | awk 'NR==2{print $2" total, "$3" used, "$4" free ("$5")"}')" \
	"$(free -h | awk '/Mem:/{print $3" used / "$2" total"}')"`
		out, err := client.Output(script)
		if err != nil {
			return err
		}
		lines := strings.Split(out, "\n")
		ui.KV([][2]string{
			{"OS", lines[0]},
			{"Uptime", lines[1]},
			{"Load", lines[2]},
			{"Disk", lines[3]},
			{"Mem", lines[4]},
		})
		return nil
	},
}

var serverSSHCmd = &cobra.Command{
	Use:     "ssh [name]",
	Aliases: []string{"shell"},
	Short:   "Open an interactive SSH shell to a server",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := resolveServerName(oneArg(args))
		client, _, err := dial(name)
		if err != nil {
			return err
		}
		defer client.Close()
		ui.Info("connected to %s — type exit to return", name)
		return client.RunInteractive()
	},
}

var serverRunCmd = &cobra.Command{
	Use:   "run [name] -- <command>",
	Short: "Run a command on a server and stream output",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := resolveServerName(args[0])
		command := strings.Join(args[1:], " ")
		if command == "" {
			return fmt.Errorf("no command given — use: sastoops server run <name> -- <command>")
		}
		client, _, err := dial(name)
		if err != nil {
			return err
		}
		defer client.Close()
		return client.Run(command)
	},
}

var serverStatusCmd = &cobra.Command{
	Use:     "status [names…]",
	Aliases: []string{"st"},
	Short:   "One-glance status for all servers (or the ones given)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := getConfig()
		if err != nil {
			return err
		}
		names := args
		if len(names) == 0 {
			names = ui.SortedKeys(c.Servers)
		}
		if len(names) == 0 {
			return fmt.Errorf("no servers configured — run: sastoops server add <name> <user>@<host>")
		}
		type result struct {
			Name  string        `json:"name"`
			Stats *monitorStats `json:"stats"`
			Error string        `json:"error,omitempty"`
		}
		results := []result{}
		for _, name := range names {
			client, _, err := dial(name)
			if err != nil {
				results = append(results, result{Name: name, Error: err.Error()})
				continue
			}
			st, err := collectStats(client)
			client.Close()
			if err != nil {
				results = append(results, result{Name: name, Error: err.Error()})
				continue
			}
			results = append(results, result{Name: name, Stats: st})
		}
		if G.JSON {
			return ui.PrintJSON(results)
		}
		t := ui.NewTable("SERVER", "OS", "CPU", "MEM", "DISK", "LOAD", "UPTIME")
		for _, r := range results {
			if r.Error != "" {
				t.Add(r.Name, ui.RedS("✘ "+r.Error))
				continue
			}
			t.Add(r.Name, r.Stats.OS, fmt.Sprintf("%.0f%%", r.Stats.CPU), fmt.Sprintf("%.0f%%", r.Stats.MemPct()), fmt.Sprintf("%.0f%%", r.Stats.DiskPct()), fmt.Sprintf("%.2f", r.Stats.Load1), r.Stats.Uptime)
		}
		t.Render()
		return nil
	},
}

var serverRebootCmd = &cobra.Command{
	Use:   "reboot [name]",
	Short: "Reboot a server (via SSH or provider API)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := resolveServerName(oneArg(args))
		c, err := getConfig()
		if err != nil {
			return err
		}
		server, err := c.GetServer(name)
		if err != nil {
			return err
		}
		viaProvider, _ := cmd.Flags().GetBool("provider")
		if viaProvider && server.ProviderID != "" && server.Provider != "" && server.Provider != "generic" {
			reg := provider.NewRegistry(c)
			p, err := reg.Get(server.Provider)
			if err != nil {
				return err
			}
			if err := p.Reboot(context.Background(), server.ProviderID); err != nil {
				return err
			}
			ui.Ok("reboot requested via %s API for %s", server.Provider, name)
			return nil
		}
		if !G.Yes && !ui.Confirm(fmt.Sprintf("reboot %s now?", name), G.Yes) {
			return fmt.Errorf("aborted")
		}
		client, _, err := dial(name)
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := client.Output(`if [ "$(id -u)" -eq 0 ]; then reboot; else sudo -n reboot; fi`)
		if err != nil {
			return fmt.Errorf("reboot: %s", out)
		}
		ui.Ok("reboot sent to %s", name)
		return nil
	},
}

var serverDeleteCmd = &cobra.Command{
	Use:     "delete <name>",
	Aliases: []string{"rm"},
	Short:   "Delete a server (tears down provider VM, or just the local record)",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		c, err := getConfig()
		if err != nil {
			return err
		}
		server, err := c.GetServer(name)
		if err != nil {
			return err
		}
		if !G.Yes && !ui.Confirm(fmt.Sprintf("delete server %s? this cannot be undone", name), G.Yes) {
			return fmt.Errorf("aborted")
		}
		if server.ProviderID != "" && server.Provider != "" && server.Provider != "generic" {
			reg := provider.NewRegistry(c)
			p, err := reg.Get(server.Provider)
			if err != nil {
				return err
			}
			if err := p.Delete(context.Background(), server.ProviderID); err != nil {
				ui.Warn("provider teardown failed: %v", err)
			} else {
				ui.Ok("provider VM %s (%s) deleted", name, server.ProviderID)
			}
		}
		delete(c.Servers, name)
		if err := c.Save(G.ConfigPath); err != nil {
			return err
		}
		ui.Ok("removed server %s from config", name)
		return nil
	},
}

func parseUserHost(s string) (string, string, error) {
	i := strings.LastIndex(s, "@")
	if i < 0 {
		return "", "", fmt.Errorf("expected user@host, got %q", s)
	}
	return s[:i], s[i+1:], nil
}

func oneArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

func waitForSSH(ip string, seconds int) {
	start := time.Now()
	for time.Since(start) < time.Duration(seconds)*time.Second {
		conn, err := netDial(ip)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func init() {
	f := serverAddCmd.Flags()
	f.IntP("port", "p", 22, "SSH port")
	f.StringP("key", "k", "", "path to SSH private key (default ~/.ssh/id_ed25519)")
	f.String("region", "", "region label (e.g. bangalore)")
	f.Bool("test", false, "verify SSH connection after adding")
	sc := serverCreateCmd.Flags()
	sc.StringP("provider", "p", "hetzner", "cloud provider (hetzner, …)")
	sc.String("type", "", "server type (e.g. cx22)")
	sc.String("region", "", "region (e.g. nbg1)")
	sc.String("image", "", "image (e.g. ubuntu-24.04)")
	sc.String("ssh-key", "", "provider SSH key name/id")
	sc.Bool("setup", false, "run base setup automatically after creation")
	serverRebootCmd.Flags().Bool("provider", false, "reboot via provider API instead of SSH")
	_ = os.Getenv
}
