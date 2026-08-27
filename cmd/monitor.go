package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var monitorCmd = &cobra.Command{
	Use:     "monitor [name]",
	Aliases: []string{"top"},
	Short:   "Live server monitoring: CPU, RAM, disk, network (--watch)",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		watch, _ := cmd.Flags().GetBool("watch")
		interval, _ := cmd.Flags().GetInt("interval")
		if interval <= 0 {
			interval = 2
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		if G.JSON && !watch {
			st, err := collectStats(client)
			if err != nil {
				return err
			}
			return ui.PrintJSON(st)
		}

		if !watch {
			st, err := collectStats(client)
			if err != nil {
				return err
			}
			renderStats(st)
			return nil
		}

		// watch mode
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, os.Interrupt)
		for {
			st, err := collectStats(client)
			if err != nil {
				ui.Error("collect: %v", err)
				time.Sleep(time.Duration(interval) * time.Second)
				continue
			}
			renderStats(st)
			select {
			case <-stop:
				return nil
			case <-time.After(time.Duration(interval) * time.Second):
			}
		}
	},
}

func renderStats(st *monitorStats) {
	fmt.Print("\033[2J\033[H")
	ui.Section(fmt.Sprintf("%s — %s", st.Hostname, st.OS))
	ui.KV([][2]string{
		{"CPU", fmt.Sprintf("%.1f%%", st.CPU)},
		{"Memory", fmt.Sprintf("%.1f%% (%s / %s)", st.MemPct(), ui.HumanBytes(st.MemUsed), ui.HumanBytes(st.MemTotal))},
		{"Disk", fmt.Sprintf("%.1f%% (%s / %s)", st.DiskPct(), ui.HumanBytes(st.DiskUsed), ui.HumanBytes(st.DiskTotal))},
		{"Load", fmt.Sprintf("%.2f / %.2f", st.Load1, st.Load5)},
		{"Uptime", st.Uptime},
		{"Network", fmt.Sprintf("RX %s · TX %s", ui.HumanBytes(st.NetRX), ui.HumanBytes(st.NetTX))},
		{"Services", fmt.Sprintf("%d running", st.RunningSvcs)},
	})
}

var healthCmd = &cobra.Command{
	Use:   "health [name]",
	Short: "Run health checks (server probes + installed app health)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		st, err := collectStats(client)
		if err != nil {
			return err
		}
		diskT, _ := cmd.Flags().GetFloat64("disk")
		memT, _ := cmd.Flags().GetFloat64("mem")
		ok, problems := st.Health(diskT, memT)

		appState, err := loadRemoteState(client, serverName)
		installed := []string{}
		if err == nil {
			for n := range appState.Apps {
				installed = append(installed, n)
			}
		}
		result := map[string]any{
			"server":    serverName,
			"ok":        ok,
			"cpu":       st.CPU,
			"mem_pct":   st.MemPct(),
			"disk_pct":  st.DiskPct(),
			"load_1":    st.Load1,
			"problems":  problems,
			"installed": installed,
		}
		if G.JSON {
			return ui.PrintJSON(result)
		}
		t := ui.NewTable("SERVER", "CPU", "MEM", "DISK", "LOAD", "HEALTH")
		health := ui.GreenS("ok")
		if !ok {
			health = ui.RedS("problems: " + fmt.Sprint(problems))
		}
		t.Add(serverName, fmt.Sprintf("%.1f%%", st.CPU), fmt.Sprintf("%.1f%%", st.MemPct()), fmt.Sprintf("%.1f%%", st.DiskPct()), fmt.Sprintf("%.2f", st.Load1), health)
		t.Render()
		if len(installed) > 0 {
			ui.Info("installed apps: %v (app health via: sastoops app status <app> %s)", installed, serverName)
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Overall status: server stats, apps, backups, security",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		st, err := collectStats(client)
		if err != nil {
			return err
		}
		appState, err := loadRemoteState(client, serverName)
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(map[string]any{
				"server":   serverName,
				"stats":    st,
				"apps":     appState.Apps,
				"recipes":  appState.Recipes,
				"backups":  appState.Backups,
				"security": securityStatus(appState.Recipes),
			})
		}
		ui.Section(serverName)
		ui.KV([][2]string{
			{"OS", st.OS},
			{"CPU / Mem / Disk", fmt.Sprintf("%.0f%% / %.0f%% / %.0f%%", st.CPU, st.MemPct(), st.DiskPct())},
			{"Uptime", st.Uptime},
		})
		ui.Section("security")
		for _, k := range ui.SortedKeys(securityStatus(appState.Recipes)) {
			v := securityStatus(appState.Recipes)[k]
			mark := ui.RedS("✘")
			if v {
				mark = ui.GreenS("✓")
			}
			fmt.Printf("  %s %s\n", mark, k)
		}
		ui.Section("apps")
		if len(appState.Apps) == 0 {
			ui.Info("none — run: sastoops app install n8n %s", serverName)
		} else {
			for _, n := range ui.SortedKeys(appState.Apps) {
				a := appState.Apps[n]
				fmt.Printf("  %s %s %s\n", ui.BoldS(n), a.Version, ui.GreenS("✓ "+a.Status))
			}
		}
		ui.Section("backups")
		if appState.Backups == nil {
			ui.Info("not configured — run: sastoops backup setup %s", serverName)
		} else {
			fmt.Printf("  %s %s · last: %s (%s)\n", appState.Backups.Engine, appState.Backups.Remote, appState.Backups.LastRun, appState.Backups.LastStatus)
		}
		return nil
	},
}

func securityStatus(recipes map[string]string) map[string]bool {
	out := map[string]bool{
		"ssh-hardened": false,
		"firewall":     false,
		"fail2ban":     false,
		"unattended":   false,
		"docker":       false,
	}
	for r := range recipes {
		switch r {
		case "base", "production":
			out["ssh-hardened"] = true
			out["firewall"] = true
			out["fail2ban"] = true
			out["unattended"] = true
			out["docker"] = true
		}
	}
	return out
}

func init() {
	monitorCmd.Flags().BoolP("watch", "w", false, "live refresh")
	monitorCmd.Flags().IntP("interval", "i", 2, "refresh interval (seconds)")
	healthCmd.Flags().Float64("disk", 85, "disk usage warning threshold %")
	healthCmd.Flags().Float64("mem", 90, "memory usage warning threshold %")
}
