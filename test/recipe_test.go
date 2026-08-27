package test

import (
	"strings"
	"testing"

	"github.com/rahulshahDEV/sastoops/internal/recipe"
	"github.com/rahulshahDEV/sastoops/internal/state"
)

func TestLoadBaseRecipe(t *testing.T) {
	r, err := recipe.Load("base")
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
	r, err := recipe.Load("production")
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

func TestLightRecipeForSmallVPS(t *testing.T) {
	r, err := recipe.Load("light")
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

func TestResolveParams(t *testing.T) {
	r, err := recipe.Load("production")
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

type fakeEngineRemote struct {
	ran []string
}

func (f *fakeEngineRemote) Output(cmd string) (string, error) {
	f.ran = append(f.ran, cmd)
	return "ok", nil
}
func (f *fakeEngineRemote) Put(data []byte, path, mode string) error { return nil }
func (f *fakeEngineRemote) ReadFile(path string) (string, error)     { return "", nil }

func TestEngineRunsModulesAndMarks(t *testing.T) {
	r, err := recipe.Load("base")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeEngineRemote{}
	st := state.New()
	e := recipe.NewEngine(fc, st, false)

	applied, err := e.Apply(r, r.ResolveParams(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != len(r.Steps) {
		t.Errorf("expected %d applied, got %d", len(r.Steps), len(applied))
	}
	if len(fc.ran) != len(r.Steps) {
		t.Errorf("expected %d module runs, got %d", len(r.Steps), len(fc.ran))
	}
	for _, s := range r.Steps {
		if st.Modules[s.ID].Version == "" {
			t.Errorf("step %s not marked", s.ID)
		}
	}
	// module command must export env params
	found := false
	for _, cmd := range fc.ran {
		if strings.Contains(cmd, "SERVEROPS_ALLOW=") {
			found = true
		}
	}
	if !found {
		t.Error("module env params not exported")
	}
}

func TestEngineIdempotent(t *testing.T) {
	r, _ := recipe.Load("base")
	fc := &fakeEngineRemote{}
	st := state.New()
	e := recipe.NewEngine(fc, st, false)

	if _, err := e.Apply(r, r.ResolveParams(nil)); err != nil {
		t.Fatal(err)
	}
	fc.ran = nil
	applied, err := e.Apply(r, r.ResolveParams(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != 0 {
		t.Errorf("second run should apply nothing, got %v", applied)
	}
	if len(fc.ran) != 0 {
		t.Errorf("second run should execute no modules, ran %d", len(fc.ran))
	}
}

func TestEngineCheckMode(t *testing.T) {
	r, _ := recipe.Load("base")
	fc := &fakeEngineRemote{}
	st := state.New()
	e := recipe.NewEngine(fc, st, true)

	applied, err := e.Apply(r, r.ResolveParams(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(applied) != len(r.Steps) {
		t.Errorf("check mode should report all steps")
	}
	if len(fc.ran) != 0 {
		t.Errorf("check mode must not run modules")
	}
	if len(st.Modules) != 0 {
		t.Errorf("check mode must not write markers")
	}
}

func TestEngineModuleChangeTriggersRerun(t *testing.T) {
	r, _ := recipe.Load("base")
	fc := &fakeEngineRemote{}
	st := state.New()
	e := recipe.NewEngine(fc, st, false)
	e.Apply(r, r.ResolveParams(nil))

	// simulate module content change: old marker version
	st.Modules["apt-update"] = state.ModuleTag{Version: "old"}
	fc.ran = nil
	applied, _ := e.Apply(r, r.ResolveParams(nil))
	if len(applied) != 1 || applied[0] != "apt-update" {
		t.Errorf("changed module should rerun only that step, got %v", applied)
	}
}
