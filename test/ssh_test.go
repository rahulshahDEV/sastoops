package test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/config"
	"github.com/rahulshahDEV/sastoops/internal/ssh"
)

func TestDialAndOutput(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	server := &config.Server{Host: host, Port: port, User: "root", KeyPath: keyPath}
	client, err := ssh.Dial(server)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	out, err := client.Output("echo hello-from-ssh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hello-from-ssh") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestPutAndReadFile(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	server := &config.Server{Host: host, Port: port, User: "root", KeyPath: keyPath}
	client, err := ssh.Dial(server)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// Put/ReadFile go through remote commands; the test server executes bash,
	// so this exercises the full write+read round trip via tmp files.
	tmp := fmt.Sprintf("/tmp/sastoops-put-test-%d", time.Now().UnixNano())
	if err := client.Put([]byte("payload-123"), tmp, "0600"); err != nil {
		t.Fatal(err)
	}
	got, err := client.ReadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "payload-123") {
		t.Errorf("round trip failed: %q", got)
	}
}

func TestReadFileMissingIsEmpty(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	server := &config.Server{Host: host, Port: port, User: "root", KeyPath: keyPath}
	client, err := ssh.Dial(server)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	out, err := client.ReadFile("/nonexistent/file-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if out != "" {
		t.Errorf("missing file should read empty, got %q", out)
	}
}

func TestDialFailsOnWrongPort(t *testing.T) {
	server := &config.Server{Host: "127.0.0.1", Port: 1, User: "root"}
	_, err := ssh.Dial(server)
	if err == nil {
		t.Fatal("expected connection failure")
	}
}

func TestOpenTunnel(t *testing.T) {
	addr, keyPath := startSSHServer(t)
	host, port := addrParts(t, addr)
	server := &config.Server{Host: host, Port: port, User: "root", KeyPath: keyPath}
	client, err := ssh.Dial(server)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	// tunnel to the ssh port itself (it accepts connections)
	listener, err := client.OpenTunnel("127.0.0.1:0", net.JoinHostPort(host, fmt.Sprintf("%d", port)))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	conn, err := net.DialTimeout("tcp", listener.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}
