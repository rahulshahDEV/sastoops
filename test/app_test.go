package test

import (
	"strings"
	"testing"

	"github.com/rahulshahDEV/sastoops/internal/app"
)

func TestLoadEmbeddedApps(t *testing.T) {
	names, err := app.All()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"n8n": false, "minecraft": false, "appwrite": false, "supabase": false}
	for _, n := range names {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for n, ok := range want {
		if !ok {
			t.Errorf("expected embedded app %s", n)
		}
	}
	// every embedded app must load AND render a compose file — catches
	// invalid YAML/templates that otherwise silently skip the app
	for _, n := range names {
		a, err := app.Load(n, "")
		if err != nil {
			t.Errorf("app %s failed to load: %v", n, err)
			continue
		}
		if _, err := a.Compose("", "latest", map[string]string{}, map[string]string{}, false); err != nil {
			t.Errorf("app %s compose failed to render: %v", n, err)
		}
	}
}

func TestComposeRenderN8N(t *testing.T) {
	a, err := app.Load("n8n", "")
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{
		"N8N_HOST":           "n8n.example.com",
		"N8N_PORT":           "5678",
		"N8N_PROTOCOL":       "https",
		"N8N_ENCRYPTION_KEY": "sekret",
	}
	compose, err := a.Compose("n8n.example.com", "1.80.1", map[string]string{"tz": "UTC"}, env, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"n8nio/n8n:1.80.1",
		"N8N_ENCRYPTION_KEY: \"sekret\"",
		"traefik.enable=true",
		"Host(`n8n.example.com`)",
		"n8n_data:/home/node/.n8n",
	} {
		if !strings.Contains(compose, want) {
			t.Errorf("compose missing %q", want)
		}
	}
}

func TestComposeRenderMinecraftNoProxy(t *testing.T) {
	a, err := app.Load("minecraft", "")
	if err != nil {
		t.Fatal(err)
	}
	compose, err := a.Compose("", "latest", map[string]string{"type": "paper", "memory": "2G", "online_mode": "true"}, map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(compose, "traefik") {
		t.Error("minecraft should not render traefik labels")
	}
	if !strings.Contains(compose, "EULA: \"TRUE\"") {
		t.Error("missing EULA")
	}
}

func TestComposeRenderN8NMemLimit(t *testing.T) {
	a, err := app.Load("n8n", "")
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]string{"N8N_HOST": "x", "N8N_ENCRYPTION_KEY": "k"}
	withLimit, err := a.Compose("x", "latest", map[string]string{"mem_limit": "512m"}, env, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(withLimit, "mem_limit: 512m") {
		t.Errorf("mem_limit param not rendered: %s", withLimit)
	}
	noLimit, err := a.Compose("x", "latest", map[string]string{}, env, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(noLimit, "mem_limit") {
		t.Error("empty mem_limit should not render")
	}
}

func TestResolveEnvSecretsPersist(t *testing.T) {
	a, err := app.Load("n8n", "")
	if err != nil {
		t.Fatal(err)
	}
	env1 := a.ResolveEnv("n8n.example.com", nil, nil)
	if env1["N8N_ENCRYPTION_KEY"] == "" {
		t.Fatal("secret not generated")
	}
	env2 := a.ResolveEnv("n8n.example.com", nil, env1)
	if env2["N8N_ENCRYPTION_KEY"] != env1["N8N_ENCRYPTION_KEY"] {
		t.Error("secret must survive re-install")
	}
	if env2["N8N_HOST"] != "n8n.example.com" {
		t.Error("domain env not applied")
	}
}

func TestEnvFileRoundTrip(t *testing.T) {
	env := map[string]string{"A": "1", "B": "two words"}
	file := app.EnvFile(env)
	parsed := app.ParseEnvFile(file)
	if parsed["A"] != "1" || parsed["B"] != "two words" {
		t.Errorf("round trip failed: %v", parsed)
	}
}

func TestUnknownAppError(t *testing.T) {
	_, err := app.Load("definitely-not-real", "")
	if err == nil || !strings.Contains(err.Error(), "unknown app") {
		t.Errorf("expected unknown app error, got %v", err)
	}
}
