package cmd

import (
	"strings"
	"testing"
)

func execute(args ...string) error {
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func TestVersionCommand(t *testing.T) {
	if err := execute("version"); err != nil {
		t.Fatal(err)
	}
}

func TestRootHelpShowsGroups(t *testing.T) {
	rootCmd.SetArgs([]string{"--help"})
	buf := &strings.Builder{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"server", "recipe", "app", "backup", "monitor", "secure"} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q", want)
		}
	}
}

func TestServerListNoConfig(t *testing.T) {
	// temp config path that doesn't exist
	oldPath := G.ConfigPath
	G.ConfigPath = "/nonexistent/sastoops.yaml"
	defer func() { G.ConfigPath = oldPath }()
	err := execute("server", "list")
	if err == nil {
		t.Fatal("expected error with no config")
	}
}

func TestServerAddValidation(t *testing.T) {
	err := execute("server", "add", "x", "notauserhost")
	if err == nil || !strings.Contains(err.Error(), "expected user@host") {
		t.Errorf("expected user@host validation error, got %v", err)
	}
}

func TestRecipeList(t *testing.T) {
	if err := execute("recipe", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestAppList(t *testing.T) {
	if err := execute("app", "list"); err != nil {
		t.Fatal(err)
	}
}

func TestAppInstallNoServer(t *testing.T) {
	oldPath := G.ConfigPath
	G.ConfigPath = "/nonexistent/sastoops.yaml"
	defer func() { G.ConfigPath = oldPath }()
	err := execute("app", "install", "n8n", "missing-server")
	if err == nil {
		t.Fatal("expected error for missing server")
	}
}

func TestUnknownCommandErrors(t *testing.T) {
	err := execute("frobnicate")
	if err == nil {
		t.Fatal("expected unknown command error")
	}
}
