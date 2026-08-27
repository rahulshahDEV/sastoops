package test

import (
	"strings"
	"testing"

	"github.com/rahulshahDEV/sastoops/internal/state"
)

func TestParseAndMarshal(t *testing.T) {
	st := state.New()
	st.MarkModule("ssh-hardening", "abc123")
	st.Apps["n8n"] = &state.AppState{Name: "n8n", Version: "latest", Status: "healthy"}
	data, err := st.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := state.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.ModuleApplied("ssh-hardening", "abc123") {
		t.Error("module marker lost")
	}
	if parsed.Apps["n8n"].Status != "healthy" {
		t.Error("app state lost")
	}
}

func TestModuleAppliedWrongVersion(t *testing.T) {
	st := state.New()
	st.MarkModule("docker", "v1")
	if st.ModuleApplied("docker", "v2") {
		t.Error("v1 marker should not satisfy v2")
	}
}

func TestSchemaTooNew(t *testing.T) {
	data := []byte(`{"schema_version": 99}`)
	_, err := state.Parse(data)
	if err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Errorf("expected schema version error, got %v", err)
	}
}

func TestLocalCacheRoundTrip(t *testing.T) {
	st := state.New()
	st.MarkModule("fail2ban", "x")
	if err := st.WriteLocalCache("testserver"); err != nil {
		t.Fatal(err)
	}
	got, err := state.ReadLocalCache("testserver")
	if err != nil {
		t.Fatal(err)
	}
	if !got.ModuleApplied("fail2ban", "x") {
		t.Error("cache round trip failed")
	}
}
