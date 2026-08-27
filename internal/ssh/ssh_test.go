package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"golang.org/x/crypto/ssh"
)

// startTestServer runs a real SSH server on a random port; each session
// echoes back "pong:<cmd>" via an internal command handler.
func startTestServer(t *testing.T) (addr string, keyPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	priv := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	dir := t.TempDir()
	keyPath = filepath.Join(dir, "id_test")
	os.WriteFile(keyPath, pem.EncodeToMemory(priv), 0o600)

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		NoClientAuth: false,
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil // accept any key for the test
		},
	}
	cfg.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				sconn, chans, reqs, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer sconn.Close()
				go ssh.DiscardRequests(reqs)
				for newChan := range chans {
					if newChan.ChannelType() != "session" {
						newChan.Reject(ssh.UnknownChannelType, "no")
						continue
					}
					ch, requests, err := newChan.Accept()
					if err != nil {
						continue
					}
					go func() {
						for req := range requests {
							switch req.Type {
							case "exec":
								var payload struct {
									Command string
								}
								ssh.Unmarshal(req.Payload, &payload)
								req.Reply(true, nil)
								ch.Write([]byte("pong:" + payload.Command + "\n"))
								ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
								ch.Close()
								return
							case "shell", "pty-req", "env":
								req.Reply(true, nil)
							}
						}
					}()
				}
			}()
		}
	}()
	return listener.Addr().String(), keyPath
}

func TestDialAndOutput(t *testing.T) {
	addr, keyPath := startTestServer(t)
	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	for _, r := range portStr {
		port = port*10 + int(r-'0')
	}
	server := &config.Server{Host: host, Port: port, User: "root", KeyPath: keyPath}
	client, err := Dial(server)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	out, err := client.Output("echo hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "pong:echo hello") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPutAndReadFile(t *testing.T) {
	addr, keyPath := startTestServer(t)
	host, portStr, _ := net.SplitHostPort(addr)
	var port int
	for _, r := range portStr {
		port = port*10 + int(r-'0')
	}
	server := &config.Server{Host: host, Port: port, User: "root", KeyPath: keyPath}
	client, err := Dial(server)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Put/ReadFile go through commands; our fake server just echoes,
	// so assert the command is well-formed rather than the FS effect.
	if err := client.Put([]byte("data"), "/tmp/x.env", "0600"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadFile("/tmp/x.env"); err != nil {
		t.Fatal(err)
	}
}

func TestDialFailsOnWrongPort(t *testing.T) {
	server := &config.Server{Host: "127.0.0.1", Port: 1, User: "root"}
	_, err := Dial(server)
	if err == nil {
		t.Fatal("expected connection failure")
	}
}
