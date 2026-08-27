package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/rahulshahDEV/sastoops/assets"
	"gopkg.in/yaml.v3"
)

type App struct {
	Name         string       `yaml:"name"`
	Version      string       `yaml:"version"`
	Category     string       `yaml:"category"`
	Description  string       `yaml:"description"`
	Homepage     string       `yaml:"homepage"`
	Requirements Requirements `yaml:"requirements"`
	Env          []EnvVar     `yaml:"env"`
	Params       []ParamDef   `yaml:"params"`
	Ports        []int        `yaml:"ports"`
	Volumes      []string     `yaml:"volumes"`
	Proxy        *Proxy       `yaml:"proxy"`
	Healthcheck  *Healthcheck `yaml:"healthcheck"`
	Lifecycle    Lifecycle    `yaml:"lifecycle"`
	Backup       BackupSpec   `yaml:"backup"`
	composeTpl   string
	overlayDir   string
}

type Requirements struct {
	OS        []string `yaml:"os"`
	Arch      []string `yaml:"arch"`
	Docker    string   `yaml:"docker"`
	MinRAM    string   `yaml:"min_ram"`
	MinDisk   string   `yaml:"min_disk"`
	PortsFree []int    `yaml:"ports_free"`
}

type EnvVar struct {
	Name    string `yaml:"name"`
	From    string `yaml:"from,omitempty"`
	Default string `yaml:"default,omitempty"`
	Secret  bool   `yaml:"secret,omitempty"`
}

type ParamDef struct {
	Name    string `yaml:"name"`
	Default string `yaml:"default"`
}

type Proxy struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
	TLS     bool `yaml:"tls"`
}

type Healthcheck struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url,omitempty"`
	Port     int    `yaml:"port,omitempty"`
	Interval string `yaml:"interval,omitempty"`
	Retries  int    `yaml:"retries"`
}

type Lifecycle struct {
	PreUpdate []string `yaml:"pre_update,omitempty"`
}

type BackupSpec struct {
	Databases []DB       `yaml:"databases"`
	Resources []Resource `yaml:"resources"`
}

type DB struct {
	Type        string `yaml:"type"`
	Container   string `yaml:"container"`
	User        string `yaml:"user"`
	PasswordRef string `yaml:"password_ref"`
	DB          string `yaml:"db"`
	Name        string `yaml:"name"`
}

type Resource struct {
	Type string `yaml:"type"`
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

func All() ([]string, error) {
	entries, err := assets.FS.ReadDir("apps")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// Load parses an app definition from embedded assets, with optional user overlay.
func Load(name, overlayDir string) (*App, error) {
	var data []byte
	var err error
	if overlayDir != "" {
		data, err = os.ReadFile(filepath.Join(overlayDir, name, "app.yaml"))
		if err == nil {
			return parseApp(name, data, overlayDir)
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	data, err = assets.FS.ReadFile("apps/" + name + "/app.yaml")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			names, _ := All()
			return nil, fmt.Errorf("unknown app %q — available: %v", name, names)
		}
		return nil, err
	}
	return parseApp(name, data, "")
}

func parseApp(name string, data []byte, overlay string) (*App, error) {
	var a App
	if err := yaml.Unmarshal(data, &a); err != nil {
		return nil, fmt.Errorf("invalid app %s: %w", name, err)
	}
	tpl, err := assets.FS.ReadFile("apps/" + name + "/compose.yaml")
	if err != nil {
		return nil, fmt.Errorf("app %s missing compose.yaml: %w", name, err)
	}
	if overlay != "" {
		if o, err := os.ReadFile(filepath.Join(overlay, name, "compose.yaml")); err == nil {
			tpl = o
		}
	}
	a.composeTpl = string(tpl)
	a.overlayDir = overlay
	return &a, nil
}

// RenderData is the template data passed to compose rendering.
type RenderData struct {
	Domain    string
	Version   string
	Port      int
	ProxyPort int
	Proxy     bool
}

// Compose renders the compose template with resolved env/params.
func (a *App) Compose(domain, version string, params, env map[string]string, proxy bool) (string, error) {
	funcs := template.FuncMap{
		"Env":    func(k string) string { return env[k] },
		"Param":  func(k string) string { return params[k] },
		"Secret": func(k string) string { return env[k] },
	}
	data := RenderData{
		Domain:    domain,
		Version:   version,
		Port:      a.Port(0),
		ProxyPort: a.ProxyPort(),
		Proxy:     proxy,
	}
	t, err := template.New(a.Name).Funcs(funcs).Parse(a.composeTpl)
	if err != nil {
		return "", fmt.Errorf("render %s compose: %w", a.Name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render %s compose: %w", a.Name, err)
	}
	return buf.String(), nil
}

// Port returns the host port for the app (default 0 = unset).
func (a *App) Port(override int) int {
	if override != 0 {
		return override
	}
	if len(a.Ports) > 0 {
		return a.Ports[0]
	}
	return 0
}

func (a *App) ProxyPort() int {
	if a.Proxy != nil && a.Proxy.Port != 0 {
		return a.Proxy.Port
	}
	return a.Port(0)
}

// ResolveEnv builds the final env map; existing values (from prior installs)
// win for secrets so they survive re-installs. New secrets are generated.
func (a *App) ResolveEnv(domain string, params map[string]string, existing map[string]string) map[string]string {
	env := map[string]string{}
	if existing != nil {
		for k, v := range existing {
			env[k] = v
		}
	}
	for _, ev := range a.Env {
		switch ev.From {
		case "domain":
			env[ev.Name] = domain
		case "param":
			env[ev.Name] = params[ev.Name]
		default:
			if _, ok := env[ev.Name]; !ok {
				if ev.Secret {
					env[ev.Name] = randHex(24)
				} else {
					env[ev.Name] = ev.Default
				}
			}
		}
	}
	return env
}

// EnvFile renders KEY=VALUE lines, sorted, secrets last (still in file, 0600).
func EnvFile(env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, env[k]))
	}
	return sb.String()
}

func ParseEnvFile(content string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			out[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return out
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
