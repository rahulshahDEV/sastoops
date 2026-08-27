package recipe

import (
	"strings"
	"testing"

	"github.com/rahulshahDEV/sastoops/internal/state"
)

type fakeRemote struct {
	ran   []string
	state string
}

func (f *fakeRemote) Output(cmd string) (string, error) {
	f.ran = append(f.ran, cmd)
	return "ok", nil
}

func (f *fakeRemote) Put(data []byte, path, mode string) error { return nil }
func (f *fakeRemote) ReadFile(path string) (string, error)     { return f.state, nil }

func TestApplyRunsModulesAndMarks(t *testing.T) {
	r, err := Load("base")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeRemote{}
	st := state.New()
	e := NewEngine(fc, st, false)

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
	// all steps should be marked
	for _, s := range r.Steps {
		if !st.ModuleApplied(s.ID, moduleVer(t, s.Module)) {
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

func TestApplyIdempotent(t *testing.T) {
	r, err := Load("base")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeRemote{}
	st := state.New()
	e := NewEngine(fc, st, false)

	if _, err := e.Apply(r, r.ResolveParams(nil)); err != nil {
		t.Fatal(err)
	}
	firstRuns := len(fc.ran)
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
	_ = firstRuns
}

func TestApplyCheckMode(t *testing.T) {
	r, err := Load("base")
	if err != nil {
		t.Fatal(err)
	}
	fc := &fakeRemote{}
	st := state.New()
	e := NewEngine(fc, st, true)

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

func TestModuleChangeTriggersRerun(t *testing.T) {
	r, _ := Load("base")
	fc := &fakeRemote{}
	st := state.New()
	e := NewEngine(fc, st, false)
	e.Apply(r, r.ResolveParams(nil))

	// simulate module content change: version differs
	st.Modules["apt-update"] = state.ModuleTag{Version: "old"}
	fc.ran = nil
	applied, _ := e.Apply(r, r.ResolveParams(nil))
	if len(applied) != 1 || applied[0] != "apt-update" {
		t.Errorf("changed module should rerun only that step, got %v", applied)
	}
}

func moduleVer(t *testing.T, m string) string {
	t.Helper()
	v, err := moduleVersionFor(m)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
