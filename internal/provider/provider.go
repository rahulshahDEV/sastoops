package provider

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/rahulshahDEV/sastoops/internal/config"
)

var ErrUnsupported = errors.New("not supported for this provider")

type Machine struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Status string `json:"status"`
	Region string `json:"region"`
	Type   string `json:"type"`
}

type CreateRequest struct {
	Name       string
	Type       string
	Region     string
	Image      string
	SSHKeyName string
	UserData   string
	Labels     map[string]string
}

type Provider interface {
	Name() string
	Create(ctx context.Context, req CreateRequest) (*Machine, error)
	List(ctx context.Context) ([]*Machine, error)
	Get(ctx context.Context, id string) (*Machine, error)
	Delete(ctx context.Context, id string) error
	Reboot(ctx context.Context, id string) error
	Rescue(ctx context.Context, id string, keys []string) error
}

// Registry constructs providers from config.
type Registry struct {
	cfg *config.Config
}

func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{cfg: cfg}
}

func (r *Registry) Providers() []string {
	names := []string{"generic"}
	for n := range r.cfg.Providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Get(name string) (Provider, error) {
	switch name {
	case "generic":
		return &Generic{}, nil
	case "hetzner":
		pc := r.cfg.Providers["hetzner"]
		if pc == nil {
			return nil, fmt.Errorf("provider hetzner not configured — add it to config (providers.hetzner)")
		}
		return &Hetzner{cfg: pc}, nil
	default:
		return nil, fmt.Errorf("unsupported provider %q — available: %v", name, r.Providers())
	}
}

// Generic is a read-only provider for servers not created by ServerOps.
type Generic struct{}

func (g *Generic) Name() string { return "generic" }
func (g *Generic) Create(ctx context.Context, req CreateRequest) (*Machine, error) {
	return nil, fmt.Errorf("generic provider cannot create servers — use a cloud provider or add an existing server: sastoops server add")
}
func (g *Generic) List(ctx context.Context) ([]*Machine, error) { return nil, nil }
func (g *Generic) Get(ctx context.Context, id string) (*Machine, error) {
	return nil, ErrUnsupported
}
func (g *Generic) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("generic provider cannot delete servers — remove it from config: sastoops server delete")
}
func (g *Generic) Reboot(ctx context.Context, id string) error { return ErrUnsupported }
func (g *Generic) Rescue(ctx context.Context, id string, keys []string) error {
	return ErrUnsupported
}
