package recipe

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/rahulshahDEV/sastoops/assets"
	"github.com/rahulshahDEV/sastoops/internal/state"
	"github.com/rahulshahDEV/sastoops/internal/ui"
)

// RemoteClient is the subset of *ssh.Client the engine needs.
type RemoteClient interface {
	Output(command string) (string, error)
	Put(data []byte, remotePath, mode string) error
	ReadFile(path string) (string, error)
}

type Engine struct {
	client RemoteClient
	state  *state.ServerState
	check  bool
	quiet  bool
}

func NewEngine(client RemoteClient, st *state.ServerState, check bool) *Engine {
	return &Engine{client: client, state: st, check: check, quiet: false}
}

// Apply runs recipe steps in order, skipping already-applied modules.
// Returns list of step ids that were applied (or would be in check mode).
func (e *Engine) Apply(r *Recipe, params map[string]string) ([]string, error) {
	applied := []string{}
	for _, step := range r.Steps {
		ver, err := e.moduleVersion(step.Module)
		if err != nil {
			return applied, err
		}
		if e.state.ModuleApplied(step.ID, ver) {
			ui.Info("step %s already applied (v%s) — skipping", step.ID, ver)
			continue
		}
		label := step.ID
		if step.Description != "" {
			label = step.Description
		}
		if e.check {
			ui.Step("would apply: %s", label)
			applied = append(applied, step.ID)
			continue
		}
		ui.Step("applying: %s", label)
		sp := r.StepParams(step, params)
		if err := e.runModule(step.Module, sp); err != nil {
			return applied, fmt.Errorf("step %s failed: %w", step.ID, err)
		}
		ui.Ok("%s", label)
		e.state.MarkModule(step.ID, ver)
		applied = append(applied, step.ID)
	}
	return applied, nil
}

func (e *Engine) moduleVersion(module string) (string, error) {
	return moduleVersionFor(module)
}

func moduleVersionFor(module string) (string, error) {
	data, err := assets.FS.ReadFile("modules/" + module + ".sh")
	if err != nil {
		return "", fmt.Errorf("module %q not found: %w", module, err)
	}
	return hashVersion(string(data)), nil
}

func (e *Engine) runModule(module string, params map[string]string) error {
	data, err := assets.FS.ReadFile("modules/" + module + ".sh")
	if err != nil {
		return err
	}
	env := buildEnv(params)
	encoded := base64.StdEncoding.EncodeToString(data)
	cmd := fmt.Sprintf("%s echo '%s' | base64 -d | bash", env, encoded)
	out, err := e.client.Output(cmd)
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

func buildEnv(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("export ")
	for k, v := range params {
		sb.WriteString(fmt.Sprintf("SERVEROPS_%s=%q ", strings.ToUpper(strings.ReplaceAll(k, "-", "_")), v))
	}
	sb.WriteString(";")
	return sb.String()
}

func hashVersion(s string) string {
	h := 0
	for _, c := range s {
		h = (h*31 + int(c)) & 0xfffffff
	}
	return fmt.Sprintf("%x", h)
}
