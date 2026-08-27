package recipe

import (
	"strings"
	"testing"
)

func TestLoadBaseRecipe(t *testing.T) {
	r, err := Load("base")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) == 0 {
		t.Fatal("base recipe has no steps")
	}
	hasDocker := false
	for _, s := range r.Steps {
		if s.Module == "docker" {
			hasDocker = true
		}
	}
	if !hasDocker {
		t.Error("base recipe must include docker module")
	}
}

func TestProductionExtendsBase(t *testing.T) {
	r, err := Load("production")
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, s := range r.Steps {
		found[s.Module] = true
	}
	for _, want := range []string{"ssh-hardening", "firewall", "fail2ban", "docker", "traefik"} {
		if !found[want] {
			t.Errorf("production recipe missing inherited module %s", want)
		}
	}
	if r.Name != "production" {
		t.Errorf("extends should keep recipe name, got %s", r.Name)
	}
}

func TestResolveParams(t *testing.T) {
	r, err := Load("production")
	if err != nil {
		t.Fatal(err)
	}
	p := r.ResolveParams(map[string]string{})
	if p["traefik_email"] != "ops@sastohost.com" {
		t.Errorf("default param missing: %v", p)
	}
	p2 := r.ResolveParams(map[string]string{"traefik_email": "x@y.z"})
	if p2["traefik_email"] != "x@y.z" {
		t.Errorf("override failed: %v", p2)
	}
}

func TestModuleVersionStable(t *testing.T) {
	v1, err := moduleVersionFor("apt-update")
	if err != nil {
		t.Fatal(err)
	}
	v2, _ := moduleVersionFor("apt-update")
	if v1 != v2 {
		t.Errorf("module version must be deterministic: %s != %s", v1, v2)
	}
}

func TestLightRecipeForSmallVPS(t *testing.T) {
	r, err := Load("light")
	if err != nil {
		t.Fatal(err)
	}
	mods := map[string]bool{}
	for _, s := range r.Steps {
		mods[s.Module] = true
	}
	if !mods["light-tune"] || !mods["swap"] {
		t.Errorf("light recipe must include light-tune and swap modules: %v", mods)
	}
}

func TestBuildEnv(t *testing.T) {
	out := buildEnv(map[string]string{"allow": "22,80", "tz": "UTC"})
	if !strings.Contains(out, "SERVEROPS_ALLOW=\"22,80\"") {
		t.Errorf("env missing: %s", out)
	}
	if !strings.Contains(out, "SERVEROPS_TZ=\"UTC\"") {
		t.Errorf("env missing: %s", out)
	}
}
