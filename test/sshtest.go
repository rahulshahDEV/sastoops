package test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

// startSSHServer runs a real SSH server on a random port that executes bash
// commands via exec. Returns addr (host:port) and the path of the client key.
func startSSHServer(t *testing.T) (addr, keyPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	priv := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	dir := t.TempDir()
	keyPath = filepath.Join(dir, "id_test")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(priv), 0o600); err != nil {
		t.Fatal(err)
	}

	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			return nil, nil
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
								var p struct{ Command string }
								ssh.Unmarshal(req.Payload, &p)
								req.Reply(true, nil)
								out, err := exec.Command("bash", "-c", p.Command).CombinedOutput()
								ch.Write(out)
								status := uint32(0)
								if err != nil {
									status = 1
								}
								ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
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

// addrParts splits host and port for a listener address.
func addrParts(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	var port int
	for _, r := range portStr {
		port = port*10 + int(r-'0')
	}
	return host, port
}
