package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

// doctorCmd checks the local environment and each server, telling beginners
// exactly what to fix and how.
var doctorCmd = &cobra.Command{
	Use:   "doctor [name]",
	Short: "Check your setup (local env + server connectivity) and show fixes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := resolveServerName(oneArg(args))
		t := ui.NewTable("CHECK", "RESULT", "HOW TO FIX")
		ok := true

		// --- local checks ---
		c, err := config.Load(G.ConfigPath)
		if err != nil {
			t.Add("config file", ui.RedS("missing"), "run: sastoops wizard  (or: sastoops server add <name> user@host)")
			ok = false
		} else {
			t.Add("config file", ui.GreenS("found"), "")
			if len(c.Servers) == 0 {
				t.Add("servers", ui.RedS("none"), "run: sastoops wizard  (or: sastoops self)")
				ok = false
			} else {
				t.Add("servers", ui.GreenS(fmt.Sprintf("%d configured", len(c.Servers))), "")
			}
		}
		keys := checkSSHKeys()
		if len(keys) == 0 {
			t.Add("ssh key", ui.RedS("none found"), "generate one: ssh-keygen -t ed25519, then: ssh-copy-id root@<server>")
			ok = false
		} else {
			t.Add("ssh key", ui.GreenS(keys[0]), "")
		}

		// --- per-server checks ---
		if c != nil {
			servers := map[string]*config.Server{}
			if name != "" {
				if s, err := c.GetServer(name); err == nil {
					servers[name] = s
				}
			} else {
				servers = c.Servers
			}
			for _, n := range ui.SortedKeys(servers) {
				s := servers[n]
				t.Add("connect "+n, checkConnect(s), "")
				client, _, err := dial(n)
				if err != nil {
					continue
				}
				out, err := client.Output(`if command -v docker >/dev/null 2>&1; then echo "docker $(docker --version | cut -d' ' -f3 | tr -d ',')"; else echo none; fi`)
				client.Close()
				if err != nil || strings.Contains(out, "none") {
					t.Add("docker on "+n, ui.RedS("not installed"), "run: sastoops recipe apply base "+n)
					ok = false
				} else {
					t.Add("docker on "+n, ui.GreenS(out), "")
				}
			}
		}

		t.Render()
		if ok {
			ui.Ok("everything looks good")
		} else {
			ui.Info("run: sastoops wizard  — it will fix the above step by step")
		}
		return nil
	},
}

func checkSSHKeys() []string {
	dir := filepath.Join(os.Getenv("HOME"), ".ssh")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var keys []string
	for _, e := range entries {
		name := e.Name()
		if (name == "id_ed25519" || name == "id_rsa" || name == "id_ecdsa") && !e.IsDir() {
			keys = append(keys, name)
		}
	}
	return keys
}

func checkConnect(s *config.Server) string {
	port := s.Port
	if port == 0 {
		port = 22
	}
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", s.Host, port), 3*time.Second)
	if err != nil {
		return ui.RedS("unreachable")
	}
	conn.Close()
	return ui.GreenS("reachable")
}
