package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/config"
)

// Hetzner is a thin REST client for the Hetzner Cloud API.
// Docs: https://docs.hetzner.cloud
type Hetzner struct {
	cfg *config.ProviderCfg
}

const hcloudAPI = "https://api.hetzner.cloud/v1"

func (h *Hetzner) Name() string { return "hetzner" }

func (h *Hetzner) token() (string, error) {
	t := h.cfg.Token()
	if t == "" {
		return "", fmt.Errorf("hetzner token missing — set providers.hetzner.token or SASTO_HETZNER_TOKEN")
	}
	return t, nil
}

func (h *Hetzner) do(ctx context.Context, method, path string, body any, out any) error {
	token, err := h.token()
	if err != nil {
		return err
	}
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, hcloudAPI+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("hetzner API %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (h *Hetzner) Create(ctx context.Context, req CreateRequest) (*Machine, error) {
	typ := req.Type
	if typ == "" {
		typ = h.cfg.DefaultType
	}
	image := req.Image
	if image == "" {
		image = h.cfg.DefaultImage
	}
	region := req.Region
	if region == "" {
		region = h.cfg.DefaultRegion
	}
	sshKey := req.SSHKeyName
	if sshKey == "" {
		sshKey = h.cfg.SSHKeyName
	}
	body := map[string]any{
		"name":        req.Name,
		"server_type": typ,
		"image":       image,
		"location":    region,
		"ssh_keys":    []string{sshKey},
	}
	if req.UserData != "" {
		body["user_data"] = req.UserData
	}
	var resp struct {
		Server struct {
			ID        int    `json:"id"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
		} `json:"server"`
	}
	if err := h.do(ctx, http.MethodPost, "/servers", body, &resp); err != nil {
		return nil, err
	}
	return &Machine{
		ID:     fmt.Sprintf("%d", resp.Server.ID),
		Name:   req.Name,
		IP:     resp.Server.PublicNet.IPv4.IP,
		Status: resp.Server.Status,
		Region: region,
		Type:   typ,
	}, nil
}

func (h *Hetzner) List(ctx context.Context) ([]*Machine, error) {
	var resp struct {
		Servers []struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
		} `json:"servers"`
	}
	if err := h.do(ctx, http.MethodGet, "/servers?per_page=50", nil, &resp); err != nil {
		return nil, err
	}
	var out []*Machine
	for _, s := range resp.Servers {
		out = append(out, &Machine{
			ID:     fmt.Sprintf("%d", s.ID),
			Name:   s.Name,
			IP:     s.PublicNet.IPv4.IP,
			Status: s.Status,
		})
	}
	return out, nil
}

func (h *Hetzner) Get(ctx context.Context, id string) (*Machine, error) {
	var resp struct {
		Server struct {
			ID        int    `json:"id"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			PublicNet struct {
				IPv4 struct {
					IP string `json:"ip"`
				} `json:"ipv4"`
			} `json:"public_net"`
		} `json:"server"`
	}
	if err := h.do(ctx, http.MethodGet, "/servers/"+id, nil, &resp); err != nil {
		return nil, err
	}
	return &Machine{
		ID:     fmt.Sprintf("%d", resp.Server.ID),
		Name:   resp.Server.Name,
		IP:     resp.Server.PublicNet.IPv4.IP,
		Status: resp.Server.Status,
	}, nil
}

func (h *Hetzner) Delete(ctx context.Context, id string) error {
	return h.do(ctx, http.MethodDelete, "/servers/"+id, nil, nil)
}

func (h *Hetzner) Reboot(ctx context.Context, id string) error {
	return h.do(ctx, http.MethodPost, "/servers/"+id+"/actions/reboot", map[string]any{}, nil)
}

func (h *Hetzner) Rescue(ctx context.Context, id string, keys []string) error {
	body := map[string]any{"type": "linux64", "ssh_keys": keys}
	return h.do(ctx, http.MethodPost, "/servers/"+id+"/actions/enable_rescue", body, nil)
}
