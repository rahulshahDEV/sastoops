package test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// Black-box CLI tests: every command runs against the real built binary with
// an isolated HOME, so they cover the exact beginner experience.

// cfgArgs returns the --config flag pair for an isolated HOME.
func cfgArgs(home string) []string {
	return []string{"--config", filepath.Join(home, "config", "sastoops", "config.yaml")}
}

func TestCLIVersion(t *testing.T) {
	out, code := run(freshHome(t), "version")
	if code != 0 || !strings.Contains(out, "sastoops") {
		t.Errorf("version failed: code=%d out=%s", code, out)
	}
}

func TestCLIBareCommandShowsWelcome(t *testing.T) {
	out, code := run(freshHome(t))
	if code != 0 {
		t.Fatalf("bare command failed: %d", code)
	}
	if !strings.Contains(out, "sastoops wizard") {
		t.Errorf("bare command should point to the wizard, got: %s", out)
	}
}

func TestCLIWizardNonTTY(t *testing.T) {
	out, code := run(freshHome(t), "wizard")
	if code != 0 {
		t.Fatalf("wizard failed: %d", code)
	}
	if !strings.Contains(out, "Non-interactive shell detected") {
		t.Errorf("wizard should print instructions in non-TTY, got: %s", out)
	}
}

func TestCLIAppList(t *testing.T) {
	out, code := run(freshHome(t), "app", "list")
	if code != 0 {
		t.Fatalf("app list failed: %d", code)
	}
	for _, want := range []string{"n8n", "minecraft", "appwrite", "supabase"} {
		if !strings.Contains(out, want) {
			t.Errorf("app list missing %q", want)
		}
	}
}

func TestCLIRecipeList(t *testing.T) {
	out, code := run(freshHome(t), "recipe", "list")
	if code != 0 {
		t.Fatalf("recipe list failed: %d", code)
	}
	for _, want := range []string{"base", "light", "production"} {
		if !strings.Contains(out, want) {
			t.Errorf("recipe list missing %q", want)
		}
	}
}

func TestCLIRecipeShow(t *testing.T) {
	out, code := run(freshHome(t), "recipe", "show", "production")
	if code != 0 || !strings.Contains(out, "traefik") {
		t.Errorf("recipe show failed: code=%d out=%s", code, out)
	}
}

func TestCLIDoctorMissingConfig(t *testing.T) {
	out, code := run(freshHome(t), "doctor")
	if code != 0 {
		t.Fatalf("doctor failed: %d", code)
	}
	if !strings.Contains(out, "config file") || !strings.Contains(out, "sastoops wizard") {
		t.Errorf("doctor should diagnose missing config, got: %s", out)
	}
}

func TestCLIServerAddValidation(t *testing.T) {
	home := freshHome(t)
	out, code := run(home, append(cfgArgs(home), "server", "add", "x", "notauserhost")...)
	if code == 0 || !strings.Contains(out, "user@host") {
		t.Errorf("expected user@host validation error, got code=%d out=%s", code, out)
	}
}

func TestCLIServerListJSON(t *testing.T) {
	home := freshHome(t)
	if out, code := run(home, append(cfgArgs(home), "server", "add", "web", "root@10.0.0.1")...); code != 0 {
		t.Fatalf("server add failed: %d %s", code, out)
	}
	out, code := run(home, append(cfgArgs(home), "--json", "server", "list")...)
	if code != 0 || !strings.Contains(out, "10.0.0.1") || !strings.Contains(out, "\"Name\": \"web\"") {
		t.Errorf("server list --json failed: code=%d out=%s", code, out)
	}
}

// TestCLIServerSetupCheck runs the hardening recipe in dry-run mode against a
// real in-process SSH server.
func TestCLIServerSetupCheck(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	home := freshHome(t)
	cfg := cfgArgs(home)

	if out, code := run(home, append(cfg, "server", "add", "demo", fmt.Sprintf("root@%s", host),
		"--port", fmt.Sprintf("%d", port), "--key", keyPath, "--test")...); code != 0 {
		t.Fatalf("server add --test failed: code=%d out=%s", code, out)
	}
	out, code := run(home, append(cfg, "server", "setup", "demo", "--check")...)
	if code != 0 {
		t.Fatalf("server setup --check failed: code=%d out=%s", code, out)
	}
	for _, want := range []string{"would apply", "ssh-hardening", "docker"} {
		if !strings.Contains(out, want) {
			t.Errorf("setup --check missing %q: %s", want, out)
		}
	}
}

// TestCLIServerRunAndStatus exercises SSH end-to-end through the binary.
func TestCLIServerRunAndStatus(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	home := freshHome(t)
	cfg := cfgArgs(home)

	if out, code := run(home, append(cfg, "server", "add", "demo", fmt.Sprintf("root@%s", host),
		"--port", fmt.Sprintf("%d", port), "--key", keyPath, "--test")...); code != 0 {
		t.Fatalf("server add failed: code=%d out=%s", code, out)
	}
	out, code := run(home, append(cfg, "server", "run", "demo", "--", "echo pong-42")...)
	if code != 0 || !strings.Contains(out, "pong-42") {
		t.Errorf("server run failed: code=%d out=%s", code, out)
	}
	out, code = run(home, append(cfg, "server", "status")...)
	if code != 0 || !strings.Contains(out, "demo") {
		t.Errorf("server status failed: code=%d out=%s", code, out)
	}
	// auto-selection: with exactly one server, "status" without args uses it
	if !strings.Contains(out, "demo") {
		t.Errorf("auto server selection failed: %s", out)
	}
}

// TestCLIAutoServerSelection: single server + no arg = implicit target.
func TestCLIAutoServerSelection(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	home := freshHome(t)
	cfg := cfgArgs(home)

	if out, code := run(home, append(cfg, "server", "add", "only", fmt.Sprintf("root@%s", host),
		"--port", fmt.Sprintf("%d", port), "--key", keyPath)...); code != 0 {
		t.Fatalf("server add failed: %d %s", code, out)
	}
	out, code := run(home, append(cfg, "server", "info")...)
	if code != 0 {
		t.Fatalf("server info (auto) failed: %d %s", code, out)
	}
	if !strings.Contains(out, "connected to only") {
		t.Errorf("auto-selected wrong server: %s", out)
	}
}

// TestCLIAppInstallNoServer: beginner error should point at the wizard.
func TestCLIAppInstallNoServer(t *testing.T) {
	home := freshHome(t)
	out, code := run(home, append(cfgArgs(home), "app", "install", "n8n")...)
	if code == 0 {
		t.Fatal("expected failure without a server")
	}
	if !strings.Contains(out, "no servers configured") || !strings.Contains(out, "wizard") {
		t.Errorf("error should guide to wizard, got: %s", out)
	}
}

// TestCLIAppInstallCheckAgainstServer: install dry-run against the test server.
func TestCLIAppInstallCheckAgainstServer(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	home := freshHome(t)
	cfg := cfgArgs(home)

	if out, code := run(home, append(cfg, "server", "add", "demo", fmt.Sprintf("root@%s", host),
		"--port", fmt.Sprintf("%d", port), "--key", keyPath)...); code != 0 {
		t.Fatalf("server add failed: %d %s", code, out)
	}
	out, code := run(home, append(cfg, "app", "install", "n8n", "demo", "--check")...)
	if code != 0 {
		t.Fatalf("app install --check failed: %d %s", code, out)
	}
	if !strings.Contains(out, "would write") {
		t.Errorf("check mode should print would-write, got: %s", out)
	}
}
