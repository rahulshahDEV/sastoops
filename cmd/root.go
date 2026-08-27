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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if G.Debug {
			ui.Info("debug mode on")
		}
	},
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

// dial resolves a server (positional or --server) and opens SSH.
func dial(name string) (*ssh.Client, *config.Server, error) {
	if name == "" {
		name = G.Server
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
	return G.Server
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
	rootCmd.AddCommand(serverCmd, recipeCmd, appCmd, backupCmd, dockerCmd, secureCmd, monitorCmd, statusCmd, healthCmd, dnsCmd)
}
