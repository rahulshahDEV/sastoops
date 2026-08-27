package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/backup"
	"github.com/rahulshahDEV/sastoops/internal/state"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:     "backup",
	Aliases: []string{"bkp"},
	Short:   "ServerOps Backup — restic/rclone backups to S3-compatible storage",
}

func init() {
	backupCmd.AddCommand(
		backupSetupCmd, backupRunCmd, backupListCmd, backupRestoreCmd,
		backupVerifyCmd, backupStatusCmd,
	)
	backupSetupCmd.Flags().String("engine", "restic", "backup engine: restic | rclone")
	backupSetupCmd.Flags().String("provider", "wasabi", "storage provider: wasabi | r2 | b2 | s3")
	backupSetupCmd.Flags().String("bucket", "", "bucket name (required)")
	backupSetupCmd.Flags().String("remote", "", "rclone remote name (default: provider)")
	backupSetupCmd.Flags().String("key-id", "", "access key id (or env: WASABI_ACCESS_KEY_ID, R2_ACCESS_KEY_ID, B2_APPLICATION_KEY_ID)")
	backupSetupCmd.Flags().String("secret", "", "secret key (or env: WASABI_SECRET_ACCESS_KEY, R2_SECRET_ACCESS_KEY, B2_APPLICATION_KEY)")
	backupSetupCmd.Flags().String("region", "", "region (defaults per provider)")
	backupSetupCmd.Flags().StringSlice("paths", nil, "extra absolute paths to back up (repeatable)")
	backupSetupCmd.Flags().StringSlice("apps", nil, "apps to back up (repeatable)")
	backupSetupCmd.Flags().String("schedule", "", "systemd timer schedule (e.g. '*-*-* 03:00:00')")
	backupRunCmd.Flags().String("job", "", "run only this job")
	backupRestoreCmd.Flags().Bool("force", false, "skip confirmation")
}

func envKeyFor(provider, kind string) string {
	switch provider {
	case "wasabi":
		if kind == "key" {
			return "WASABI_ACCESS_KEY_ID"
		}
		return "WASABI_SECRET_ACCESS_KEY"
	case "r2":
		if kind == "key" {
			return "R2_ACCESS_KEY_ID"
		}
		return "R2_SECRET_ACCESS_KEY"
	case "b2":
		if kind == "key" {
			return "B2_APPLICATION_KEY_ID"
		}
		return "B2_APPLICATION_KEY"
	default:
		if kind == "key" {
			return "S3_ACCESS_KEY_ID"
		}
		return "S3_SECRET_ACCESS_KEY"
	}
}

func envOrFlag(provider, kind, flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return lookupEnv(envKeyFor(provider, kind))
}

var backupSetupCmd = &cobra.Command{
	Use:   "setup [name]",
	Short: "Configure backups on a server (engine, storage, jobs)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		engine, _ := cmd.Flags().GetString("engine")
		provider, _ := cmd.Flags().GetString("provider")
		bucket, _ := cmd.Flags().GetString("bucket")
		remoteName, _ := cmd.Flags().GetString("remote")
		keyID := envOrFlag(provider, "key", keyIDFlag(cmd))
		secret := envOrFlag(provider, "secret", secretFlag(cmd))
		region, _ := cmd.Flags().GetString("region")
		paths, _ := cmd.Flags().GetStringSlice("paths")
		appsList, _ := cmd.Flags().GetStringSlice("apps")
		schedule, _ := cmd.Flags().GetString("schedule")

		// beginner-friendly prompts when flags are missing
		if bucket == "" {
			bucket = ui.Prompt("Bucket name (create it in your provider first)", "")
		}
		if keyID == "" {
			keyID = ui.Prompt("Access key id", "")
		}
		if secret == "" {
			secret = ui.Prompt("Secret key", "")
		}
		if bucket == "" {
			return fmt.Errorf("--bucket is required (e.g. sastoops-backups-%s)", serverName)
		}
		if keyID == "" || secret == "" {
			return fmt.Errorf("storage credentials required — pass --key-id/--secret or set %s/%s",
				envKeyFor(provider, "key"), envKeyFor(provider, "secret"))
		}
		if region == "" {
			region = backup.ProviderRegion(provider)
		}
		if remoteName == "" {
			remoteName = provider
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		return runBackupSetup(client, serverName, engine, provider, bucket, remoteName, keyID, secret, region, paths, appsList, schedule)
	},
}

// runBackupSetup is the shared backup configuration flow (command + wizard).
func runBackupSetup(client *sshClient, serverName, engine, provider, bucket, remoteName, keyID, secret, region string, paths, appsList []string, schedule string) error {
	remote := remoteName
	prefix := serverName
	repo := fmt.Sprintf("s3:%s", bucket)

	ui.Step("installing %s engine", engine)
	if err := backup.InstallEngine(client, engine); err != nil {
		return err
	}
	ui.Ok("engine ready")

	if engine == "restic" {
		env := backup.RenderResticEnv(keyID, secret, region)
		endpoint := backup.Endpoint(provider)
		if endpoint != "" {
			repo = fmt.Sprintf("s3:%s/%s/%s", strings.TrimPrefix(endpoint, "https://"), bucket, prefix)
		} else {
			repo = fmt.Sprintf("s3:%s/%s", bucket, prefix)
		}
		if err := backup.WriteEnv(client, env); err != nil {
			return err
		}
		ui.Step("initializing restic repository")
		if err := backup.InitResticRepo(client, repo); err != nil {
			return err
		}
	} else {
		cfg := backup.RenderRcloneConfig(remote, provider, keyID, secret)
		if err := client.Put([]byte(cfg), backup.RcloneCfg, "0600"); err != nil {
			return err
		}
		ui.Step("testing rclone connectivity")
		if err := backup.RcloneTest(client, remote); err != nil {
			return err
		}
		ui.Ok("rclone remote %s reachable", remote)
	}

	// collect app paths from installed apps
	st, err := loadRemoteState(client, serverName)
	if err != nil {
		return err
	}
	jobApps := appsList
	jobPaths := append([]string{}, paths...)
	if len(jobApps) == 0 {
		for aName := range st.Apps {
			jobApps = append(jobApps, aName)
		}
	}

	jobs := &backup.Jobs{Engine: engine, Remote: remote, Repo: repo}
	jobs.Jobs = []backup.JobSpec{{
		Name:        "daily",
		Apps:        jobApps,
		Paths:       jobPaths,
		Schedule:    schedule,
		KeepLast:    7,
		KeepDaily:   30,
		KeepMonthly: 12,
	}}
	if err := backup.SaveJobs(client, jobs); err != nil {
		return err
	}
	ui.Ok("jobs saved: %s (retention 7d/30d/12m)", serverName)

	if schedule != "" {
		unit := fmt.Sprintf(`[Unit]
Description=ServerOps backup %s
[Service]
Type=oneshot
ExecStart=/bin/bash -c 'set -a; . /var/lib/serverops/secrets/backup.env; set +a; restic -r %s backup /var/lib/serverops --tag system 2>/dev/null || true'
`, serverName, repo)
		timer := fmt.Sprintf(`[Unit]
Description=ServerOps backup timer %s
[Timer]
OnCalendar=%s
Persistent=true
[Install]
WantedBy=timers.target
`, serverName, schedule)
		if err := client.Put([]byte(unit), "/etc/systemd/system/serverops-backup.service", "0644"); err != nil {
			return err
		}
		if err := client.Put([]byte(timer), "/etc/systemd/system/serverops-backup.timer", "0644"); err != nil {
			return err
		}
		client.Output(`if [ "$(id -u)" -eq 0 ]; then S=''; else S='sudo -n'; fi; $S systemctl daemon-reload && $S systemctl enable --now serverops-backup.timer`)
		ui.Ok("systemd timer installed: %s", schedule)
	}
	ui.Info("next: sastoops backup run %s", serverName)
	return nil
}

var backupRunCmd = &cobra.Command{
	Use:   "run [name]",
	Short: "Run backup jobs now (DB dumps + restic/rclone)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		jobFilter, _ := cmd.Flags().GetString("job")
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		jobs, err := backup.LoadJobs(client)
		if err != nil {
			return err
		}
		for _, j := range jobs.Jobs {
			if jobFilter != "" && j.Name != jobFilter {
				continue
			}
			ui.Section("job: %s", j.Name)
			// 1. app db dumps
			for _, aName := range j.Apps {
				a, err := loadApp(aName)
				if err != nil {
					ui.Warn("%v", err)
					continue
				}
				client.Output(fmt.Sprintf("mkdir -p %q", state.DumpDir))
				if err := runAppDumps(client, a); err != nil {
					ui.Warn("dump %s: %v", aName, err)
				}
			}
			// 2. backup paths
			paths := append([]string{}, state.DumpDir)
			paths = append(paths, j.Paths...)
			if jobs.Engine == "restic" {
				ui.Step("restic backup → %s", jobs.Repo)
				if _, err := backup.ResticBackup(client, jobs.Repo, paths, []string{"daily", serverName}); err != nil {
					ui.Error("%v", err)
				} else {
					ui.Ok("snapshot created")
				}
				if err := backup.ResticForget(client, jobs.Repo, j); err != nil {
					ui.Warn("retention prune: %v", err)
				}
			} else {
				ui.Step("rclone sync → %s:%s/", jobs.Remote, serverName)
				if out, err := backup.RcloneBackup(client, jobs.Remote, serverName, paths); err != nil {
					ui.Error("%v", err)
				} else {
					ui.Ok("synced: %s", lastLine(out))
				}
			}
			st, err := loadRemoteState(client, serverName)
			if err == nil {
				if st.Backups == nil {
					st.Backups = &state.BackupState{}
				}
				st.Backups.LastRun = time.Now().UTC().Format(time.RFC3339)
				st.Backups.LastStatus = "ok"
				saveRemoteState(client, st)
			}
		}
		return nil
	},
}

var backupListCmd = &cobra.Command{
	Use:     "list [name]",
	Aliases: []string{"ls"},
	Short:   "List backup snapshots/files",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		jobs, err := backup.LoadJobs(client)
		if err != nil {
			return err
		}
		if jobs.Engine == "restic" {
			snaps, err := backup.ResticSnapshots(client, jobs.Repo)
			if err != nil {
				return err
			}
			if G.JSON {
				return ui.PrintJSON(snaps)
			}
			if len(snaps) == 0 {
				ui.Info("no snapshots yet — run: sastoops backup run %s", serverName)
				return nil
			}
			t := ui.NewTable("ID", "DATE", "TAGS")
			for _, s := range snaps {
				t.Add(s.ID, s.Summary, s.Tags)
			}
			t.Render()
			return nil
		}
		out, err := backup.RcloneList(client, jobs.Remote, serverName)
		if err != nil {
			return err
		}
		if strings.TrimSpace(out) == "" {
			ui.Info("no files yet — run: sastoops backup run %s", serverName)
			return nil
		}
		fmt.Println(out)
		return nil
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore [name] <snapshot-id|latest> [dest]",
	Short: "Restore a backup snapshot to a directory (staged, then confirm)",
	Args:  cobra.MaximumNArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName("")
		rest := args
		if len(rest) > 0 {
			if c, err := getConfig(); err == nil {
				if _, ok := c.Servers[rest[0]]; ok {
					serverName = rest[0]
					rest = rest[1:]
				}
			}
		}
		if len(rest) < 1 {
			return fmt.Errorf("usage: sastoops backup restore [name] <snapshot-id|latest> [dest]")
		}
		snapshot := rest[0]
		dest := "/var/lib/serverops/restore"
		if len(rest) > 1 {
			dest = rest[1]
		}
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		jobs, err := backup.LoadJobs(client)
		if err != nil {
			return err
		}
		force, _ := cmd.Flags().GetBool("force")
		if !force && !G.Yes && !ui.Confirm(fmt.Sprintf("restore %s to %s on %s? this overwrites files there", snapshot, dest, serverName), G.Yes) {
			return fmt.Errorf("aborted")
		}
		ui.Step("restoring…")
		if jobs.Engine == "restic" {
			if err := backup.ResticRestore(client, jobs.Repo, snapshot, dest); err != nil {
				return err
			}
		} else {
			if err := backup.RcloneRestore(client, jobs.Remote, serverName, dest); err != nil {
				return err
			}
		}
		ui.Ok("restored to %s", dest)
		return nil
	},
}

var backupVerifyCmd = &cobra.Command{
	Use:   "verify [name]",
	Short: "Verify backup integrity (restic check / rclone check)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		jobs, err := backup.LoadJobs(client)
		if err != nil {
			return err
		}
		ui.Step("verifying with %s…", jobs.Engine)
		start := time.Now()
		var verr error
		if jobs.Engine == "restic" {
			verr = backup.ResticVerify(client, jobs.Repo)
		} else {
			verr = backup.RcloneVerify(client, jobs.Remote, serverName, state.DumpDir)
		}
		if verr != nil {
			ui.Error("verify failed: %v", verr)
			return verr
		}
		ui.Ok("backup integrity verified (%.0fs)", time.Since(start).Seconds())
		return nil
	},
}

var backupStatusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show backup configuration and last run status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serverName := resolveServerName(oneArg(args))
		client, _, err := dial(serverName)
		if err != nil {
			return err
		}
		defer client.Close()
		jobs, err := backup.LoadJobs(client)
		if err != nil {
			return err
		}
		st, err := loadRemoteState(client, serverName)
		if err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(map[string]any{"engine": jobs.Engine, "remote": jobs.Remote, "repo": jobs.Repo, "backups": st.Backups})
		}
		ui.KV([][2]string{
			{"Engine", jobs.Engine},
			{"Remote", jobs.Remote},
			{"Repo", jobs.Repo},
			{"Last run", st.Backups.LastRun},
			{"Last status", st.Backups.LastStatus},
		})
		t := ui.NewTable("JOB", "APPS", "PATHS", "SCHEDULE", "RETENTION")
		for _, j := range jobs.Jobs {
			t.Add(j.Name, strings.Join(j.Apps, ","), strings.Join(j.Paths, ","), j.Schedule, fmt.Sprintf("%d/%d/%d", j.KeepLast, j.KeepDaily, j.KeepMonthly))
		}
		t.Render()
		return nil
	},
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return s
	}
	return lines[len(lines)-1]
}

func keyIDFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("key-id")
	return v
}

func secretFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("secret")
	return v
}
