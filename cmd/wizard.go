package cmd

import (
	"fmt"
	"strings"

	"github.com/rahulshahDEV/sastoops/internal/app"
	"github.com/rahulshahDEV/sastoops/internal/config"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

// wizardCmd guides a beginner through: server -> hardening -> app -> backups.
var wizardCmd = &cobra.Command{
	Use:     "wizard",
	Aliases: []string{"setup", "quickstart", "init"},
	Short:   "Interactive guided setup — server, hardening, apps, backups",
	Long:    `Guided setup for anyone — no flags needed.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !ui.IsTTY() {
			fmt.Println("Non-interactive shell detected. Run these instead:")
			fmt.Println("  1. sastoops self                                  # register this machine")
			fmt.Println("  2. sastoops server setup <name>                   # harden + Docker")
			fmt.Println("  3. sastoops app install n8n <name> --domain <d>   # install an app")
			fmt.Println("  4. sastoops backup setup <name> --provider wasabi --bucket <b>")
			return nil
		}
		welcome()
		serverName, err := wizardServer()
		if err != nil {
			return err
		}
		if serverName == "" {
			ui.Info("no server — finishing wizard. Run: sastoops wizard")
			return nil
		}
		if ui.Confirm(fmt.Sprintf("harden %s now? (SSH, firewall, fail2ban, Docker — safe, re-runnable)", serverName), G.Yes) {
			if err := runServerSetup(serverName, "base"); err != nil {
				ui.Warn("setup had issues: %v", err)
			}
		}
		wizardApp(serverName)
		wizardBackup(serverName)
		summary(serverName)
		return nil
	},
}

func welcome() {
	fmt.Printf("\n%s\n", ui.BoldS("sastoops — SastoHost ServerOps"))
	fmt.Printf("%s\n", ui.DimS("Your VPS, managed in minutes. No agents, no flags required."))
}

func wizardServer() (string, error) {
	c, err := config.LoadOrNew(G.ConfigPath)
	if err != nil {
		return "", err
	}
	if len(c.Servers) == 0 {
		ui.Section("1. Connect a server")
		choice := ui.Select("Where should apps run?", []string{
			"This machine (recommended for a VPS you are on)",
			"Another server — I'll type its IP",
		})
		switch choice {
		case 0:
			return wizardSelf()
		case 1:
			return wizardAddServer()
		default:
			return "", nil
		}
	}
	if len(c.Servers) == 1 {
		for n := range c.Servers {
			ui.Section("1. Server")
			ui.Ok("using configured server %s", n)
			return n, nil
		}
	}
	ui.Section("1. Server")
	choice := ui.Select("Which server?", ui.SortedKeys(c.Servers))
	if choice < 0 {
		return "", nil
	}
	return ui.SortedKeys(c.Servers)[choice], nil
}

func wizardSelf() (string, error) {
	name := ui.Prompt("Name for this machine", "")
	if name == "" {
		name = "vps"
	}
	user := ui.Prompt("SSH user", "")
	port := 22
	if p := ui.Prompt("SSH port", "22"); p != "" && p != "22" {
		fmt.Sscanf(p, "%d", &port)
	}
	if err := registerSelf(name, user, port, "", "", "", false); err != nil {
		return "", err
	}
	return name, nil
}

func wizardAddServer() (string, error) {
	userHost := ui.Prompt("Server as user@host (e.g. root@1.2.3.4)", "")
	if userHost == "" {
		return "", nil
	}
	name := ui.Prompt("Name for this server", strings.Split(userHost, "@")[1])
	key := ui.Prompt("SSH key path (leave empty for default)", "")
	user, host, err := parseUserHost(userHost)
	if err != nil {
		return "", err
	}
	c, err := config.LoadOrNew(G.ConfigPath)
	if err != nil {
		return "", err
	}
	c.Servers[name] = &config.Server{Host: host, User: user, Port: 22, KeyPath: key, Provider: "generic"}
	if err := c.Save(G.ConfigPath); err != nil {
		return "", err
	}
	ui.Ok("added server %s (%s@%s)", name, user, host)
	return name, nil
}

func wizardApp(serverName string) {
	ui.Section("2. Install an app")
	if !ui.Confirm("install an app now? (n8n, minecraft, appwrite, supabase…)", false) {
		return
	}
	names, err := app.All()
	if err != nil {
		ui.Warn("%v", err)
		return
	}
	labels := make([]string, 0, len(names))
	for _, n := range names {
		a, err := app.Load(n, appOverlayDir())
		if err == nil {
			labels = append(labels, fmt.Sprintf("%s — %s", n, a.Description))
		} else {
			labels = append(labels, n)
		}
	}
	choice := ui.Select("Which app?", labels)
	if choice < 0 {
		return
	}
	aName := names[choice]
	domain := ui.Prompt("Public domain (e.g. n8n.example.com — empty to skip HTTPS)", "")
	if err := installAppWizard(aName, serverName, domain); err != nil {
		ui.Warn("install had issues: %v", err)
	}
}

func wizardBackup(serverName string) {
	ui.Section("3. Backups")
	if !ui.Confirm("set up encrypted backups? (restic -> Wasabi/R2/B2)", false) {
		ui.Info("you can enable them later: sastoops backup setup %s", serverName)
		return
	}
	provider := "wasabi"
	if p := ui.Prompt("Storage provider (wasabi/r2/b2)", "wasabi"); p != "" {
		provider = p
	}
	bucket := ui.Prompt("Bucket name (create it in your provider first)", "")
	if bucket == "" {
		ui.Warn("bucket required — skipping. Run: sastoops backup setup %s", serverName)
		return
	}
	keyID := ui.Prompt("Access key id", "")
	secret := ui.Prompt("Secret key", "")
	client, _, err := dial(serverName)
	if err != nil {
		ui.Warn("%v", err)
		return
	}
	defer client.Close()
	if err := runBackupSetup(client, serverName, "restic", provider, bucket, "", keyID, secret, "", nil, nil, ""); err != nil {
		ui.Warn("backup setup had issues: %v", err)
	}
}

func summary(serverName string) {
	ui.Section("4. All done")
	ui.Ok("your server is ready to use")
	fmt.Printf("\n%s\n", ui.BoldS("Useful commands:"))
	fmt.Printf("  sastoops status %s                    # see everything at a glance\n", serverName)
	fmt.Printf("  sastoops ssh %s                       # open a shell\n", serverName)
	fmt.Printf("  sastoops monitor %s --watch           # live monitoring\n", serverName)
	fmt.Printf("  sastoops backup run %s                # back up now\n", serverName)
	fmt.Printf("  sastoops doctor %s                    # check health of the setup\n", serverName)
}
