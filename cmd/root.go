package cmd

import (
	"fmt"
	"os"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"github.com/rahulshahDEV/sastoops/internal/ssh"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

type GlobalFlags struct {
	ConfigPath string
	Server     string
	JSON       bool
	Quiet      bool
	Verbose    bool
	Debug      bool
	Yes        bool
	Check      bool
}

var G GlobalFlags

var rootCmd = &cobra.Command{
	Use:   "sastoops",
	Short: "SastoHost ServerOps — manage any VPS from your terminal",
	Long: `sastoops — lightweight server management CLI by SastoHost (https://sasto.host)

ServerOps Cloud     SSH management of any VPS (add, ssh, run, status)
ServerOps Security  hardening: SSH, firewall, fail2ban (recipes)
ServerOps Deploy    Docker + apps: n8n, minecraft, appwrite, supabase
ServerOps Backup    restic/rclone -> Wasabi, R2, B2 (setup, run, restore)
ServerOps Monitor   CPU / RAM / disk / network / health
ServerOps Registry  one-command app installs (app list, app install)`,
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		welcomeMenu()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if G.Debug {
			ui.Info("debug mode on")
		}
	},
}

// welcomeMenu is shown when sastoops runs with no subcommand — beginner-first.
func welcomeMenu() {
	fmt.Printf("\n%s\n", ui.BoldS("sastoops — SastoHost ServerOps"))
	fmt.Printf("%s\n\n", ui.DimS("New here? Start with the guided setup:"))
	fmt.Printf("  %s  sastoops wizard\n\n", ui.GreenS("→"))
	fmt.Printf("%s\n", ui.BoldS("Common commands"))
	fmt.Printf("  sastoops self                    register the machine you are on\n")
	fmt.Printf("  sastoops server list             show your servers\n")
	fmt.Printf("  sastoops server setup <name>     harden + Docker\n")
	fmt.Printf("  sastoops app install             pick an app interactively\n")
	fmt.Printf("  sastoops status <name>           everything at a glance\n")
	fmt.Printf("  sastoops doctor                  check your setup\n")
	fmt.Printf("\n%s\n", ui.DimS("Run 'sastoops --help' for the full command list."))
}

func init() {
	pf := rootCmd.PersistentFlags()
	pf.StringVar(&G.ConfigPath, "config", "", "path to config file (default ~/.config/sastoops/config.yaml)")
	pf.StringVarP(&G.Server, "server", "s", "", "target server name (alternative to positional arg)")
	pf.BoolVar(&G.JSON, "json", false, "machine-readable JSON output")
	pf.BoolVarP(&G.Quiet, "quiet", "q", false, "only show errors")
	pf.BoolVar(&G.Verbose, "verbose", false, "extra detail")
	pf.BoolVar(&G.Debug, "debug", false, "debug output")
	pf.BoolVarP(&G.Yes, "yes", "y", false, "skip confirmation prompts")
	pf.BoolVar(&G.Check, "check", false, "dry-run: show what would change")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}
}

func getConfig() (*config.Config, error) {
	return config.Load(G.ConfigPath)
}

// dial resolves a server (positional, --server, or the only one configured) and opens SSH.
func dial(name string) (*ssh.Client, *config.Server, error) {
	if name == "" {
		name = G.Server
	}
	if name == "" {
		name = autoServerName()
	}
	if name == "" {
		return nil, nil, fmt.Errorf("no server given — run: sastoops wizard  (or: sastoops self)")
	}
	c, err := getConfig()
	if err != nil {
		return nil, nil, err
	}
	server, err := c.GetServer(name)
	if err != nil {
		return nil, nil, err
	}
	client, err := ssh.Dial(server)
	if err != nil {
		return nil, nil, err
	}
	return client, server, nil
}

func resolveServerName(arg string) string {
	if arg != "" {
		return arg
	}
	if G.Server != "" {
		return G.Server
	}
	if n := autoServerName(); n != "" {
		return n
	}
	return ""
}

// autoServerName returns the only configured server, if there is exactly one.
func autoServerName() string {
	c, err := getConfig()
	if err != nil {
		return ""
	}
	if len(c.Servers) == 1 {
		for n := range c.Servers {
			return n
		}
	}
	return ""
}

// promptServerName lets the user pick a server interactively (TTY only).
func promptServerName() (string, error) {
	c, err := config.LoadOrNew(G.ConfigPath)
	if err != nil {
		return "", err
	}
	if len(c.Servers) == 0 {
		return "", fmt.Errorf("no servers configured — run: sastoops wizard  (or: sastoops self / sastoops server add <name> user@host)")
	}
	names := ui.SortedKeys(c.Servers)
	if len(names) == 1 {
		return names[0], nil
	}
	choice := ui.Select("Which server?", names)
	if choice < 0 {
		return "", nil
	}
	return names[choice], nil
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if G.JSON {
			ui.PrintJSON(map[string]string{"name": "sastoops", "version": config.Version})
			return
		}
		fmt.Printf("sastoops %s (SastoHost ServerOps)\n", config.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(serverCmd, recipeCmd, appCmd, backupCmd, dockerCmd, secureCmd, monitorCmd, statusCmd, healthCmd, dnsCmd, wizardCmd, doctorCmd)
}
