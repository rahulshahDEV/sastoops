package recipe

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/rahulshahDEV/sastoops/assets"
	"gopkg.in/yaml.v3"
)

type Recipe struct {
	Name        string            `yaml:"name"`
	Version     string            `yaml:"version"`
	Description string            `yaml:"description"`
	Extends     string            `yaml:"extends,omitempty"`
	Params      map[string]string `yaml:"params,omitempty"`
	Steps       []Step            `yaml:"steps"`
}

type Step struct {
	ID          string            `yaml:"id"`
	Module      string            `yaml:"module"`
	Description string            `yaml:"description,omitempty"`
	Params      map[string]string `yaml:"params,omitempty"`
}

func All() ([]string, error) {
	entries, err := assets.FS.ReadDir("recipes")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return names, nil
}

func Load(name string) (*Recipe, error) {
	data, err := assets.FS.ReadFile("recipes/" + name + ".yaml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			names, _ := All()
			return nil, fmt.Errorf("unknown recipe %q — available: %v", name, names)
		}
		return nil, err
	}
	var r Recipe
	if err := yaml.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("invalid recipe %s: %w", name, err)
	}
	if r.Extends != "" {
		parent, err := Load(r.Extends)
		if err != nil {
			return nil, err
		}
		merged := *parent
		merged.Name = r.Name
		merged.Version = r.Version
		merged.Description = r.Description
		merged.Steps = append(merged.Steps, r.Steps...)
		if merged.Params == nil {
			merged.Params = map[string]string{}
		}
		for k, v := range r.Params {
			merged.Params[k] = v
		}
		return &merged, nil
	}
	return &r, nil
}

// ResolveParams resolves {{param key}} placeholders and merges overrides.
func (r *Recipe) ResolveParams(overrides map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range r.Params {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	for k, v := range out {
		out[k] = strings.ReplaceAll(v, "{{param "+k+"}}", out[k])
	}
	return out
}

// StepParams merges recipe params + overrides into step params.
func (r *Recipe) StepParams(s Step, params map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range params {
		out[k] = v
	}
	for k, v := range s.Params {
		out[k] = v
	}
	return out
}
