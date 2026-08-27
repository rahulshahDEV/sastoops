package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const Version = "0.2.0"

type Config struct {
	Global    GlobalConfig            `yaml:"global"`
	Servers   map[string]*Server      `yaml:"servers"`
	Providers map[string]*ProviderCfg `yaml:"providers"`
	Backup    *BackupConfig           `yaml:"backup,omitempty"`
}

type GlobalConfig struct {
	SSHTimeout  int `yaml:"ssh_timeout_sec,omitempty"`
	Concurrency int `yaml:"concurrency,omitempty"`
}

type Server struct {
	Host       string   `yaml:"host"`
	Port       int      `yaml:"port,omitempty"`
	User       string   `yaml:"user"`
	KeyPath    string   `yaml:"key_path,omitempty"`
	Password   string   `yaml:"password,omitempty"`
	Provider   string   `yaml:"provider,omitempty"`
	ProviderID string   `yaml:"provider_id,omitempty"`
	Region     string   `yaml:"region,omitempty"`
	Type       string   `yaml:"type,omitempty"`
	Tags       []string `yaml:"tags,omitempty"`
}

type ProviderCfg struct {
	TokenEnv      string `yaml:"token_env,omitempty"`
	TokenValue    string `yaml:"token,omitempty"`
	DefaultType   string `yaml:"default_type,omitempty"`
	DefaultImage  string `yaml:"default_image,omitempty"`
	DefaultRegion string `yaml:"default_region,omitempty"`
	SSHKeyName    string `yaml:"ssh_key,omitempty"`
}

type BackupConfig struct {
	Engine   string          `yaml:"engine,omitempty"`   // restic|rclone
	Provider string          `yaml:"provider,omitempty"` // wasabi|r2|b2|s3
	Bucket   string          `yaml:"bucket,omitempty"`
	Region   string          `yaml:"region,omitempty"`
	Endpoint string          `yaml:"endpoint,omitempty"`
	Jobs     map[string]*Job `yaml:"jobs,omitempty"`
}

type Job struct {
	Paths     []string `yaml:"paths,omitempty"`
	Apps      []string `yaml:"apps,omitempty"`
	Schedule  string   `yaml:"schedule,omitempty"`
	Retention string   `yaml:"retention,omitempty"`
}

func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(os.Getenv("HOME"), ".config", "sastoops", "config.yaml")
	}
	return filepath.Join(dir, "sastoops", "config.yaml")
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no config at %s — run: sastoops server add <name> <user>@<host>", path)
		}
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	if c.Servers == nil {
		c.Servers = map[string]*Server{}
	}
	if c.Providers == nil {
		c.Providers = map[string]*ProviderCfg{}
	}
	return &c, nil
}

func LoadOrNew(path string) (*Config, error) {
	c, err := Load(path)
	if err == nil {
		return c, nil
	}
	if strings.Contains(err.Error(), "no config at") {
		return &Config{Servers: map[string]*Server{}, Providers: map[string]*ProviderCfg{}}, nil
	}
	return nil, err
}

func (c *Config) Save(path string) error {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) GetServer(name string) (*Server, error) {
	if s, ok := c.Servers[name]; ok {
		return s, nil
	}
	names := make([]string, 0, len(c.Servers))
	for n := range c.Servers {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no servers configured — run: sastoops server add <name> <user>@<host>")
	}
	return nil, fmt.Errorf("unknown server %q — known: %v", name, names)
}

// Token resolves a provider token from config or env.
func (p *ProviderCfg) Token() string {
	if p.TokenValue != "" {
		return p.TokenValue
	}
	if p.TokenEnv != "" {
		return os.Getenv(p.TokenEnv)
	}
	return ""
}
