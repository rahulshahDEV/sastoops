package ssh

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type Client struct {
	*ssh.Client
	server *config.Server
}

func Dial(server *config.Server) (*Client, error) {
	port := server.Port
	if port == 0 {
		port = 22
	}
	addr := net.JoinHostPort(server.Host, fmt.Sprintf("%d", port))

	auths := []ssh.AuthMethod{}
	if server.Password != "" {
		auths = append(auths, ssh.Password(server.Password))
	}
	keyPath := server.KeyPath
	if keyPath == "" {
		keyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519")
	}
	if _, err := os.Stat(keyPath); err == nil {
		if signer, err := loadKey(keyPath); err == nil {
			auths = append(auths, ssh.PublicKeys(signer))
		}
	}
	if ag := sshAgent(); ag != nil {
		auths = append(auths, ssh.PublicKeysCallback(ag.Signers))
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("no auth method for %s (set key_path or password in config)", server.Host)
	}

	cfg := &ssh.ClientConfig{
		User:            server.User,
		Auth:            auths,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	conn, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, fmt.Errorf("ssh %s@%s: %w", server.User, addr, err)
	}
	return &Client{Client: conn, server: server}, nil
}

func loadKey(path string) (ssh.Signer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(data)
}

func sshAgent() agent.Agent {
	conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	if err != nil {
		return nil
	}
	return agent.NewClient(conn)
}

// Run executes a command remotely, streaming output to local stdout/stderr.
func (c *Client) Run(command string) error {
	sess, err := c.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr
	sess.Stdin = os.Stdin
	return sess.Run(command)
}

// Output runs a command remotely and returns trimmed combined output.
func (c *Client) Output(command string) (string, error) {
	sess, err := c.NewSession()
	if err != nil {
		return "", err
	}
	defer sess.Close()
	out, err := sess.CombinedOutput(command)
	return strings.TrimSpace(string(out)), err
}

// RunInteractive opens an interactive PTY shell.
func (c *Client) RunInteractive() error {
	sess, err := c.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	sess.Stdout = os.Stdout
	sess.Stderr = os.Stderr
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	go io.Copy(stdin, os.Stdin)

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	term := os.Getenv("TERM")
	if term == "" {
		term = "xterm-256color"
	}
	if err := sess.RequestPty(term, 80, 24, modes); err != nil {
		return err
	}
	if err := sess.Shell(); err != nil {
		return err
	}
	return sess.Wait()
}

// Put writes a file remotely with given permissions using base64 over SSH.
func (c *Client) Put(data []byte, remotePath string, mode string) error {
	encoded := base64.StdEncoding.EncodeToString(data)
	cmd := fmt.Sprintf("mkdir -p $(dirname %q) && echo '%s' | base64 -d > %q && chmod %s %q",
		remotePath, encoded, remotePath, mode, remotePath)
	_, err := c.Output(cmd)
	return err
}

// ReadFile reads a remote file ("" if missing).
func (c *Client) ReadFile(remotePath string) (string, error) {
	out, err := c.Output(fmt.Sprintf("cat %q 2>/dev/null || true", remotePath))
	if err != nil {
		return "", err
	}
	return out, nil
}

// Exists checks remote path existence.
func (c *Client) Exists(remotePath string) (bool, error) {
	out, err := c.Output(fmt.Sprintf("[ -e %q ] && echo yes || echo no", remotePath))
	if err != nil {
		return false, err
	}
	return out == "yes", nil
}

// OpenTunnel forwards a local port to a remote endpoint.
func (c *Client) OpenTunnel(localAddr, remoteAddr string) (net.Listener, error) {
	listener, err := net.Listen("tcp", localAddr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				remote, err := c.Client.Dial("tcp", remoteAddr)
				if err != nil {
					conn.Close()
					return
				}
				go func() { io.Copy(remote, conn) }()
				io.Copy(conn, remote)
				remote.Close()
				conn.Close()
			}()
		}
	}()
	return listener, nil
}
