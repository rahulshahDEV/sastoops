package cmd

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"github.com/rahulshahDEV/sastoops/internal/ui"
	"github.com/spf13/cobra"
)

// selfCmd registers the machine the CLI is running on as a ServerOps server —
// the "easy peasy" path: install on any VPS, run `sastoops self`, done.
var selfCmd = &cobra.Command{
	Use:   "self [name]",
	Short: "Register this machine as a ServerOps server (no flags needed)",
	Long: `Register the current machine as a ServerOps server.

Detects user, SSH key and public IP automatically, so on any fresh VPS:

  curl -fsSL https://raw.githubusercontent.com/rahulshahDEV/sastoops/main/scripts/install.sh | sh
  sastoops self
  sastoops server setup <name>`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := oneArg(args)
		if name == "" {
			host, err := os.Hostname()
			if err != nil || host == "" {
				host = "vps"
			}
			name = strings.Split(host, ".")[0]
		}
		user, _ := cmd.Flags().GetString("user")
		if user == "" {
			user = os.Getenv("USER")
		}
		port, _ := cmd.Flags().GetInt("port")
		if port == 0 {
			port = 22
		}
		key, _ := cmd.Flags().GetString("key")
		if key == "" {
			key = defaultSSHKey()
		}
		region, _ := cmd.Flags().GetString("region")
		provider, _ := cmd.Flags().GetString("provider")

		ip := publicIP()
		if ip == "" {
			ip = localIP()
		}
		if ip == "" {
			ui.Warn("could not detect public IP — using 127.0.0.1")
			ip = "127.0.0.1"
		}

		c, err := config.LoadOrNew(G.ConfigPath)
		if err != nil {
			return err
		}
		if _, exists := c.Servers[name]; exists {
			return fmt.Errorf("server %q already exists — pick another name or remove it first", name)
		}
		c.Servers[name] = &config.Server{
			Host:     ip,
			Port:     port,
			User:     user,
			KeyPath:  key,
			Region:   region,
			Provider: provider,
		}
		if err := c.Save(G.ConfigPath); err != nil {
			return err
		}
		ui.Ok("registered this machine as server %q (%s@%s)", name, user, ip)

		if test, _ := cmd.Flags().GetBool("test"); test {
			client, _, err := dial(name)
			if err != nil {
				ui.Warn("SSH to self failed (is sshd running and your key authorized?): %v", err)
				ui.Info("you can still manage this machine later from another machine: sastoops ssh %s", name)
			} else {
				client.Close()
				ui.Ok("SSH connection to self verified")
			}
		}
		ui.Info("next:")
		fmt.Printf("  sastoops server setup %s     # harden + Docker (idempotent)\n", name)
		fmt.Printf("  sastoops app install n8n %s  # install an app\n", name)
		fmt.Printf("  sastoops backup setup %s     # enable encrypted backups\n", name)
		return nil
	},
}

func defaultSSHKey() string {
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		p := filepath.Join(os.Getenv("HOME"), ".ssh", name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func publicIP() string {
	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range []string{"https://ifconfig.me/ip", "https://ipv4.icanhazip.com", "https://api.ipify.org"} {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		ip := strings.TrimSpace(string(b))
		// prefer IPv4 — SSH on v6-only addresses is often blocked by providers
		if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() != nil {
			return ip
		}
	}
	return ""
}

func localIP() string {
	out, err := exec.Command("sh", "-c", "hostname -I 2>/dev/null | awk '{print $1}'").Output()
	if err == nil {
		if ip := strings.TrimSpace(string(out)); net.ParseIP(ip) != nil {
			return ip
		}
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

func init() {
	f := selfCmd.Flags()
	f.String("user", "", "SSH user (default: current user)")
	f.IntP("port", "p", 22, "SSH port")
	f.StringP("key", "k", "", "path to SSH private key (default: ~/.ssh/id_ed25519)")
	f.String("region", "", "region label (e.g. bangalore)")
	f.String("provider", "generic", "provider label (generic, hetzner, …)")
	f.Bool("test", false, "verify SSH connection to self after registering")
	rootCmd.AddCommand(selfCmd)
}
