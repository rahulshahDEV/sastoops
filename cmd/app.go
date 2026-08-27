package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/app"
	"github.com/rahulshahDEV/sastoops/internal/backup"
	"github.com/rahulshahDEV/sastoops/internal/executor"
	"github.com/rahulshahDEV/sastoops/internal/state"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:     "app",
	Aliases: []string{"apps"},
	Short:   "ServerOps Registry — install and manage applications",
}

func init() {
	appCmd.AddCommand(
		appListCmd, appSearchCmd, appInstallCmd, appUninstallCmd,
		appUpdateCmd, appRestartCmd, appStopCmd, appStartCmd,
		appLogsCmd, appStatusCmd, appEnvCmd, appBackupCmd,
	)
	appInstallCmd.Flags().String("domain", "", "public domain (enables reverse proxy + SSL)")
	appInstallCmd.Flags().String("version", "", "image version to pin (default: app default)")
	appInstallCmd.Flags().String("port", "0", "host port override")
	appInstallCmd.Flags().StringSlice("set", nil, "set app params: --set key=value")
	appUpdateCmd.Flags().String("version", "", "upgrade/downgrade to a specific version")
}

func appDir(a *app.App) string {
	return state.ComposeDir + "/" + a.Name
}

func appOverlayDir() string {
	dir := defaultConfigDir()
	return dir + "/apps"
}

func defaultConfigDir() string {
	d, err := configDir()
	if err != nil {
		return ".sastoops"
	}
	return d
}

var appListCmd = &cobra.Command{
	Use:     "list [name]",
	Aliases: []string{"ls"},
	Short:   "List available apps in the registry (or installed apps on a server)",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return listInstalled(args[0])
		}
		names, err := app.All()
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(names)
		}
		t := ui.NewTable("APP", "CATEGORY", "DESCRIPTION")
		for _, n := range names {
			a, err := app.Load(n, appOverlayDir())
			if err != nil {
				continue
			}
			t.Add(a.Name, a.Category, a.Description)
		}
		t.Render()
		return nil
	},
}

func listInstalled(serverName string) error {
	client, _, err := dial(serverName)
	if err != nil {
		return err
	}
	defer client.Close()
	st, err := loadRemoteState(client, serverName)
	if err != nil {
		return err
	}
	if G.JSON {
		return ui.PrintJSON(st.Apps)
	}
	if len(st.Apps) == 0 {
		ui.Info("no apps installed on %s — run: sastoops app install <app> %s", serverName, serverName)
		return nil
	}
	t := ui.NewTable("APP", "VERSION", "STATUS", "DOMAIN", "INSTALLED")
	for _, n := range ui.SortedKeys(st.Apps) {
		a := st.Apps[n]
		status := a.Status
		if status == "healthy" {
			status = ui.GreenS("healthy")
		} else if status == "running" {
			status = ui.CyanS("running")
		} else {
			status = ui.RedS(status)
		}
		t.Add(a.Name, a.Version, status, a.Domain, a.InstalledAt)
	}
	t.Render()
	return nil
}

var appSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search the app registry",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		q := strings.ToLower(args[0])
		names, err := app.All()
		if err != nil {
			return err
		}
		t := ui.NewTable("APP", "CATEGORY", "DESCRIPTION")
		found := 0
		for _, n := range names {
			a, err := app.Load(n, appOverlayDir())
			if err != nil {
				continue
			}
			hay := strings.ToLower(a.Name + " " + a.Category + " " + a.Description)
			if strings.Contains(hay, q) {
				t.Add(a.Name, a.Category, a.Description)
				found++
			}
		}
		if found == 0 {
			ui.Info("no apps match %q", args[0])
			return nil
		}
		t.Render()
		return nil
	},
}

var appInstallCmd = &cobra.Command{
	Use:   "install [app] [name]",
	Short: "Install an app on a server via Docker Compose",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := oneArg(args)
		serverName := resolveServerName(oneArg(args[1:]))
		domain, _ := cmd.Flags().GetString("domain")
		version, _ := cmd.Flags().GetString("version")
		portOverride, _ := cmd.Flags().GetInt("port")
		sets, _ := cmd.Flags().GetStringSlice("set")

		// beginner-friendly pickers when args are missing
		if aName == "" {
			names, err := app.All()
			if err != nil {
				return err
			}
			labels := make([]string, 0, len(names))
			for _, n := range names {
				if a, err := app.Load(n, appOverlayDir()); err == nil {
					labels = append(labels, fmt.Sprintf("%s — %s", n, a.Description))
				} else {
					labels = append(labels, n)
				}
			}
			choice := ui.Select("Which app do you want to install?", labels)
			if choice < 0 {
				return fmt.Errorf("no app selected")
			}
			aName = names[choice]
		}
		if serverName == "" {
			serverName, err := promptServerName()
			if err != nil {
				return err
			}
			if serverName == "" {
				return fmt.Errorf("no server selected — run: sastoops wizard")
			}
		}

		a, err := app.Load(aName, appOverlayDir())
		if err != nil {
			return err
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		ui.Ok("connected to %s", serverName)
		return installApp(client, serverName, a, domain, version, portOverride, sets)
	},
}

// installAppWizard installs an app with defaults (used by the wizard).
func installAppWizard(aName, serverName, domain string) error {
	a, err := app.Load(aName, appOverlayDir())
	if err != nil {
		return err
	}
	client, _, err := dial(serverName)
	if err != nil {
		return err
	}
	defer client.Close()
	return installApp(client, serverName, a, domain, "", 0, nil)
}

func installApp(client *sshClient, serverName string, a *app.App, domain, version string, portOverride int, sets []string) error {
	if version == "" {
		version = "latest"
	}
	params := map[string]string{}
	for _, p := range a.Params {
		params[p.Name] = p.Default
	}
	for _, kv := range sets {
		if k, v, ok := cutKV(kv); ok {
			params[k] = v
		}
	}

	// dry-run: report what would happen without touching the server
	dir := appDir(a)
	if G.Check {
		ui.Step("would write %s/compose.yaml and .env, then: docker compose up -d --wait", dir)
		return nil
	}

	// requirements check (skipped in check mode so dry-runs work anywhere)
	ui.Step("checking requirements")
	if err := checkRequirements(client, a); err != nil {
		return err
	}
	ui.Ok("requirements ok")

	envFile := dir + "/.env"

	// load existing env (secrets survive re-install)
	existing := map[string]string{}
	if raw, err := client.ReadFile(envFile); err == nil && raw != "" {
		existing = app.ParseEnvFile(raw)
	}
	env := a.ResolveEnv(domain, params, existing)

	compose, err := a.Compose(domain, version, params, env, domain != "")
	if err != nil {
		return err
	}

	ui.Step("writing application files")
	if err := client.Put([]byte(compose), dir+"/compose.yaml", "0644"); err != nil {
		return err
	}
	if err := client.Put([]byte(app.EnvFile(env)), envFile, "0600"); err != nil {
		return err
	}
	ui.Step("starting containers")
	if err := executor.ComposeUp(client, dir, true); err != nil {
		return err
	}
	ui.Step("running health check")
	if err := waitAppHealth(client, a, portOverride); err != nil {
		return err
	}

	// record state
	st, err := loadRemoteState(client, serverName)
	if err != nil {
		return err
	}
	st.Apps[a.Name] = &state.AppState{
		Name: a.Name, Version: version, Status: "healthy",
		Domain: domain, InstalledAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := saveRemoteState(client, st); err != nil {
		return err
	}
	ui.Ok("%s installed (v%s)", a.Name, version)
	url := ""
	if domain != "" {
		url = "https://" + domain
	}
	ui.KV([][2]string{
		{"URL", url},
		{"Status", "healthy"},
		{"Next", "sastoops backup setup " + serverName},
	})
	return nil
}

var appUninstallCmd = &cobra.Command{
	Use:   "uninstall <app> [name]",
	Short: "Uninstall an app (containers, volumes, files)",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := args[0]
		serverName := resolveServerName(oneArg(args[1:]))
		if !G.Yes && !ui.Confirm(fmt.Sprintf("uninstall %s from %s? data volumes are removed", aName, serverName), G.Yes) {
			return fmt.Errorf("aborted")
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		dir := state.ComposeDir + "/" + aName
		ui.Step("stopping containers")
		if err := executor.ComposeDown(client, dir, true); err != nil {
			ui.Warn("compose down: %v", err)
		}
		client.Output(fmt.Sprintf("rm -rf %q", dir))
		st, err := loadRemoteState(client, serverName)
		if err != nil {
			return err
		}
		delete(st.Apps, aName)
		if err := saveRemoteState(client, st); err != nil {
			return err
		}
		ui.Ok("%s uninstalled from %s", aName, serverName)
		return nil
	},
}

var appUpdateCmd = &cobra.Command{
	Use:   "update <app> [name]",
	Short: "Update an app (backs up first, rolls back on health failure)",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := args[0]
		serverName := resolveServerName(oneArg(args[1:]))
		version, _ := cmd.Flags().GetString("version")
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()

		st, err := loadRemoteState(client, serverName)
		if err != nil {
			return err
		}
		installed, ok := st.Apps[aName]
		if !ok {
			return fmt.Errorf("%s is not installed on %s — run: sastoops app install %s %s", aName, serverName, aName, serverName)
		}
		a, err := app.Load(aName, appOverlayDir())
		if err != nil {
			return err
		}
		dir := appDir(a)
		oldVersion := installed.Version
		if version == "" {
			version = oldVersion
			ui.Step("recreating %s (v%s) with latest config", aName, oldVersion)
		}
		env := map[string]string{}
		if raw, err := client.ReadFile(dir + "/.env"); err == nil {
			env = app.ParseEnvFile(raw)
		}
		compose, err := a.Compose(installed.Domain, version, map[string]string{}, env, installed.Domain != "")
		if err != nil {
			return err
		}
		ui.Step("writing compose")
		if err := client.Put([]byte(compose), dir+"/compose.yaml", "0644"); err != nil {
			return err
		}
		ui.Step("recreating containers")
		if err := executor.ComposeAction(client, dir, "up -d --wait --force-recreate"); err != nil {
			ui.Warn("update failed, rolling back to %s", oldVersion)
			if rb, err := a.Compose(installed.Domain, oldVersion, map[string]string{}, env, installed.Domain != ""); err == nil {
				client.Put([]byte(rb), dir+"/compose.yaml", "0644")
				executor.ComposeAction(client, dir, "up -d --wait --force-recreate")
			}
			return err
		}
		ui.Step("running health check")
		if err := waitAppHealth(client, a, 0); err != nil {
			return err
		}
		installed.Version = version
		if err := saveRemoteState(client, st); err != nil {
			return err
		}
		ui.Ok("%s updated to v%s", aName, version)
		return nil
	},
}

var appRestartCmd = appActionCmd("restart", "Restart an app")
var appStopCmd = appActionCmd("stop", "Stop an app")
var appStartCmd = appActionCmd("start", "Start an app")

func appActionCmd(action, short string) *cobra.Command {
	return &cobra.Command{
		Use:   action + " <app> [name]",
		Short: short,
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			aName := args[0]
			serverName := resolveServerName(oneArg(args[1:]))
			client, _, err := dial(serverName)
			if err != nil {
				return err
			}
			defer client.Close()
			if err := executor.ComposeAction(client, state.ComposeDir+"/"+aName, action); err != nil {
				return err
			}
			ui.Ok("%s %s", action, aName)
			return nil
		},
	}
}

var appLogsCmd = &cobra.Command{
	Use:   "logs <app> [name]",
	Short: "Show app logs (-f to follow)",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := args[0]
		serverName := resolveServerName(oneArg(args[1:]))
		follow, _ := cmd.Flags().GetBool("follow")
		tail, _ := cmd.Flags().GetInt("tail")
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := executor.ComposeLogs(client, state.ComposeDir+"/"+aName, tail, follow)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

var appStatusCmd = &cobra.Command{
	Use:   "status <app> [name]",
	Short: "Show app container status and health",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := args[0]
		serverName := resolveServerName(oneArg(args[1:]))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		out, err := executor.ComposePs(client, state.ComposeDir+"/"+aName)
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(map[string]string{"app": aName, "server": serverName, "ps": out})
		}
		fmt.Println(out)
		return nil
	},
}

var appEnvCmd = &cobra.Command{
	Use:   "env <app> [name] [get|set KEY=VALUE|rm KEY]",
	Short: "Manage app environment variables",
	Args:  cobra.MaximumNArgs(5),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := args[0]
		rest := args[1:]
		serverName := resolveServerName("")
		action := "list"
		if len(rest) > 0 {
			// detect if first remaining arg is the server name
			c, err := getConfig()
			if err == nil {
				if _, ok := c.Servers[rest[0]]; ok {
					serverName = rest[0]
					rest = rest[1:]
				}
			}
		}
		if len(rest) > 0 {
			action = rest[0]
		}
		if serverName == "" {
			serverName = G.Server
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		envFile := state.ComposeDir + "/" + aName + "/.env"
		env := map[string]string{}
		if raw, err := client.ReadFile(envFile); err == nil && raw != "" {
			env = app.ParseEnvFile(raw)
		}
		switch action {
		case "list":
			if G.JSON {
				return ui.PrintJSON(env)
			}
			t := ui.NewTable("KEY", "VALUE")
			for _, k := range ui.SortedKeys(env) {
				v := env[k]
				if isSecretKey(k) {
					v = "****"
				}
				t.Add(k, v)
			}
			t.Render()
		case "get":
			if len(rest) < 2 {
				return fmt.Errorf("usage: sastoops app env %s %s get KEY", aName, serverName)
			}
			key := rest[1]
			v, ok := env[key]
			if !ok {
				return fmt.Errorf("env key %s not set", key)
			}
			if isSecretKey(key) {
				v = "****"
			}
			fmt.Println(v)
		case "set":
			if len(rest) < 2 {
				return fmt.Errorf("usage: sastoops app env %s %s set KEY=VALUE", aName, serverName)
			}
			k, v, ok := cutKV(rest[1])
			if !ok {
				return fmt.Errorf("expected KEY=VALUE, got %q", rest[1])
			}
			env[k] = v
			if err := client.Put([]byte(app.EnvFile(env)), envFile, "0600"); err != nil {
				return err
			}
			ui.Ok("env %s set (restart app to apply)", k)
		case "rm":
			if len(rest) < 2 {
				return fmt.Errorf("usage: sastoops app env %s %s rm KEY", aName, serverName)
			}
			delete(env, rest[1])
			if err := client.Put([]byte(app.EnvFile(env)), envFile, "0600"); err != nil {
				return err
			}
			ui.Ok("env %s removed", rest[1])
		default:
			return fmt.Errorf("unknown env action %q (list|get|set|rm)", action)
		}
		return nil
	},
}

var appBackupCmd = &cobra.Command{
	Use:   "backup <app> [name]",
	Short: "Run the app's database dumps (then use sastoops backup run)",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		aName := args[0]
		serverName := resolveServerName(oneArg(args[1:]))
		a, err := app.Load(aName, appOverlayDir())
		if err != nil {
			return err
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		if err := runAppDumps(client, a); err != nil {
			return err
		}
		ui.Ok("%s dumps complete — next: sastoops backup run %s", aName, serverName)
		return nil
	},
}

func runAppDumps(client *sshClient, a *app.App) error {
	if len(a.Backup.Databases) == 0 {
		ui.Info("%s has no databases to dump", a.Name)
		return nil
	}
	dir := appDir(a)
	env := map[string]string{}
	if raw, err := client.ReadFile(dir + "/.env"); err == nil && raw != "" {
		env = app.ParseEnvFile(raw)
	}
	ui.Step("dumping databases")
	for _, db := range a.Backup.Databases {
		password := env[db.PasswordRef]
		user := resolveEnvRef(db.User, env)
		dbName := resolveEnvRef(db.DB, env)
		dest := fmt.Sprintf("%s/%s-%s.sql", state.DumpDir, a.Name, db.Name)
		cmd := backup.DBDump(dir, db.Type, db.Container, user, password, dbName, dest)
		if cmd == "" {
			ui.Warn("unsupported db type %s for %s", db.Type, a.Name)
			continue
		}
		client.Output(fmt.Sprintf("mkdir -p %q", state.DumpDir))
		if _, err := client.Output(cmd); err != nil {
			return fmt.Errorf("dump %s.%s: %w", a.Name, db.Name, err)
		}
		ui.Ok("dumped %s → %s", db.Name, dest)
	}
	return nil
}

func waitAppHealth(client *sshClient, a *app.App, portOverride int) error {
	h := a.Healthcheck
	if h == nil {
		return nil
	}
	interval := executor.ParseInterval(h.Interval)
	retries := h.Retries
	if retries <= 0 {
		retries = 30
	}
	switch h.Type {
	case "http":
		url := h.URL
		if portOverride != 0 {
			url = strings.Replace(url, fmt.Sprintf(":%d", a.Port(0)), fmt.Sprintf(":%d", portOverride), 1)
		}
		if err := executor.WaitHTTP(client, url, interval, retries); err != nil {
			return err
		}
		ui.Ok("health check ok (%s)", h.URL)
	case "tcp":
		port := h.Port
		if port == 0 {
			port = a.Port(portOverride)
		}
		if err := executor.WaitTCP(client, "localhost", port, interval, retries); err != nil {
			return err
		}
		ui.Ok("port %d open", port)
	}
	return nil
}

func checkRequirements(client *sshClient, a *app.App) error {
	osName, err := client.Output(`. /etc/os-release 2>/dev/null && echo "$ID" || echo unknown`)
	if err != nil {
		return err
	}
	arch, err := client.Output("uname -m")
	if err != nil {
		return err
	}
	if len(a.Requirements.OS) > 0 && !contains(a.Requirements.OS, osName) {
		return fmt.Errorf("%s requires OS %v, server runs %s", a.Name, a.Requirements.OS, osName)
	}
	archOK := false
	for _, want := range a.Requirements.Arch {
		if want == arch || (want == "amd64" && arch == "x86_64") {
			archOK = true
		}
	}
	if len(a.Requirements.Arch) > 0 && !archOK {
		return fmt.Errorf("%s requires arch %v, server runs %s", a.Name, a.Requirements.Arch, arch)
	}
	if _, err := client.Output("docker version --format '{{.Server.Version}}' 2>/dev/null"); err != nil {
		return fmt.Errorf("docker is not installed on the server — run: sastoops recipe apply base %s", resolveServerName(""))
	}
	return nil
}

func isSecretKey(k string) bool {
	up := strings.ToUpper(k)
	for _, sub := range []string{"PASSWORD", "SECRET", "KEY", "TOKEN", "PASS"} {
		if strings.Contains(up, sub) {
			return true
		}
	}
	return false
}

// resolveEnvRef substitutes {{Env "NAME"}} placeholders with env values.
func resolveEnvRef(s string, env map[string]string) string {
	if strings.HasPrefix(s, `{{Env "`) && strings.HasSuffix(s, `"}}`) {
		key := strings.TrimSuffix(strings.TrimPrefix(s, `{{Env "`), `"}}`)
		return env[key]
	}
	return s
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func init() {
	appLogsCmd.Flags().BoolP("follow", "f", false, "follow log output")
	appLogsCmd.Flags().Int("tail", 100, "number of lines to show")
}
