package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

// ServerOps DNS — Cloudflare API (zone records).
// Token: CF_API_TOKEN env var, or providers.cloudflare.token in config.

var dnsCmd = &cobra.Command{
	Use:   "dns",
	Short: "ServerOps DNS — manage DNS records via Cloudflare",
}

func init() {
	dnsCmd.AddCommand(dnsRecordsCmd, dnsAddCmd)
}

func cfToken() string {
	if t := os.Getenv("CF_API_TOKEN"); t != "" {
		return t
	}
	c, err := getConfig()
	if err != nil {
		return ""
	}
	if p := c.Providers["cloudflare"]; p != nil {
		return p.Token()
	}
	return ""
}

type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
	Proxied bool   `json:"proxied"`
}

func cfAPI(ctx context.Context, method, path, token string, body, out any) error {
	client := &http.Client{Timeout: 20 * time.Second}
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = strings.NewReader(string(data))
	}
	req, err := http.NewRequestWithContext(ctx, method, "https://api.cloudflare.com/client/v4"+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var wrapper struct {
		Success bool              `json:"success"`
		Errors  []json.RawMessage `json:"errors"`
		Result  json.RawMessage   `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return err
	}
	if !wrapper.Success {
		return fmt.Errorf("cloudflare API error: %s", wrapper.Errors)
	}
	if out != nil {
		return json.Unmarshal(wrapper.Result, out)
	}
	return nil
}

func findZone(ctx context.Context, token, domain string) (*cfZone, error) {
	var zones []cfZone
	if err := cfAPI(ctx, http.MethodGet, "/zones?name="+domain+"&status=active", token, nil, &zones); err != nil {
		return nil, err
	}
	if len(zones) == 0 {
		return nil, fmt.Errorf("no active Cloudflare zone for %q", domain)
	}
	return &zones[0], nil
}

var dnsRecordsCmd = &cobra.Command{
	Use:     "records <domain>",
	Aliases: []string{"ls"},
	Short:   "List DNS records for a domain",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := cfToken()
		if token == "" {
			return fmt.Errorf("no Cloudflare token — set CF_API_TOKEN or providers.cloudflare.token")
		}
		ctx := context.Background()
		zone, err := findZone(ctx, token, args[0])
		if err != nil {
			return err
		}
		var records []cfRecord
		if err := cfAPI(ctx, http.MethodGet, "/zones/"+zone.ID+"/dns_records?per_page=100", token, nil, &records); err != nil {
			return err
		}
		if G.JSON {
			return ui.PrintJSON(records)
		}
		t := ui.NewTable("TYPE", "NAME", "VALUE", "PROXIED")
		for _, r := range records {
			proxied := "no"
			if r.Proxied {
				proxied = ui.GreenS("yes")
			}
			t.Add(r.Type, r.Name, r.Content, proxied)
		}
		t.Render()
		return nil
	},
}

var dnsAddCmd = &cobra.Command{
	Use:   "add <domain> <type> <name> <value>",
	Short: "Create a DNS record (A, AAAA, CNAME, TXT, MX…)",
	Args:  cobra.ExactArgs(4),
	RunE: func(cmd *cobra.Command, args []string) error {
		token := cfToken()
		if token == "" {
			return fmt.Errorf("no Cloudflare token — set CF_API_TOKEN or providers.cloudflare.token")
		}
		domain, rtype, name, value := args[0], strings.ToUpper(args[1]), args[2], args[3]
		ttl, _ := cmd.Flags().GetInt("ttl")
		proxied, _ := cmd.Flags().GetBool("proxied")
		ctx := context.Background()
		zone, err := findZone(ctx, token, domain)
		if err != nil {
			return err
		}
		full := name
		if !strings.HasSuffix(full, "."+domain) && name != "@" {
			full = name + "." + domain
		}
		if name == "@" {
			full = domain
		}
		body := map[string]any{
			"type": rtype, "name": full, "content": value, "ttl": ttl, "proxied": proxied,
		}
		var created cfRecord
		if err := cfAPI(ctx, http.MethodPost, "/zones/"+zone.ID+"/dns_records", token, body, &created); err != nil {
			return err
		}
		ui.Ok("%s %s → %s (ttl %d, proxied %v)", rtype, full, value, created.TTL, created.Proxied)
		return nil
	},
}

func init() {
	dnsAddCmd.Flags().Int("ttl", 120, "TTL seconds")
	dnsAddCmd.Flags().Bool("proxied", false, "proxy through Cloudflare")
}
