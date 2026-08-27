package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	c := &Config{
		Servers: map[string]*Server{
			"test": {Host: "1.2.3.4", User: "root", Port: 22, Provider: "generic"},
		},
		Providers: map[string]*ProviderCfg{
			"hetzner": {TokenEnv: "SASTO_HETZNER_TOKEN", DefaultType: "cx22"},
		},
	}
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Servers["test"].Host != "1.2.3.4" {
		t.Errorf("server host lost: %+v", got.Servers["test"])
	}
	if got.Providers["hetzner"].DefaultType != "cx22" {
		t.Error("provider config lost")
	}
}

func TestGetServerUnknown(t *testing.T) {
	c := &Config{Servers: map[string]*Server{"a": {Host: "1.1.1.1"}}}
	if _, err := c.GetServer("b"); err == nil {
		t.Error("expected error for unknown server")
	}
}

func TestTokenResolution(t *testing.T) {
	os.Setenv("SASTO_TEST_TOKEN", "tok123")
	defer os.Unsetenv("SASTO_TEST_TOKEN")
	p := &ProviderCfg{TokenEnv: "SASTO_TEST_TOKEN"}
	if p.Token() != "tok123" {
		t.Error("env token resolution failed")
	}
	p2 := &ProviderCfg{TokenValue: "direct"}
	if p2.Token() != "direct" {
		t.Error("direct token failed")
	}
}
