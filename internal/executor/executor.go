package executor

import (
	"fmt"
	"strings"
	"time"
)

// Client is the subset of *ssh.Client the executor needs.
type Client interface {
	Output(command string) (string, error)
	Put(data []byte, remotePath, mode string) error
	ReadFile(path string) (string, error)
}

func ComposeUp(c Client, dir string, wait bool) error {
	cmd := fmt.Sprintf("cd %q && docker compose up -d", dir)
	if wait {
		cmd += " --wait"
	}
	out, err := c.Output(cmd)
	if err != nil {
		return fmt.Errorf("compose up: %s", out)
	}
	return nil
}

func ComposeDown(c Client, dir string, volumes bool) error {
	cmd := fmt.Sprintf("cd %q && docker compose down", dir)
	if volumes {
		cmd += " -v"
	}
	out, err := c.Output(cmd)
	if err != nil {
		return fmt.Errorf("compose down: %s", out)
	}
	return nil
}

func ComposeAction(c Client, dir, action string) error {
	out, err := c.Output(fmt.Sprintf("cd %q && docker compose %s", dir, action))
	if err != nil {
		return fmt.Errorf("compose %s: %s", action, out)
	}
	return nil
}

func ComposePs(c Client, dir string) (string, error) {
	out, err := c.Output(fmt.Sprintf("cd %q && docker compose ps --format 'table {{.Name}}\t{{.Status}}\t{{.Ports}}'", dir))
	if err != nil {
		return "", err
	}
	return out, nil
}

func ComposeLogs(c Client, dir string, tail int, follow bool) (string, error) {
	cmd := fmt.Sprintf("cd %q && docker compose logs --tail %d", dir, tail)
	if follow {
		cmd += " -f"
	}
	return c.Output(cmd)
}

func ComposeExec(c Client, dir, service, command string) (string, error) {
	return c.Output(fmt.Sprintf("cd %q && docker compose exec -T %s %s", dir, service, command))
}

// WaitHTTP polls an HTTP URL on the server until ok or retries exhausted.
func WaitHTTP(c Client, url string, interval time.Duration, retries int) error {
	for i := 0; i < retries; i++ {
		out, err := c.Output(fmt.Sprintf("curl -fsS -o /dev/null -w '%%{http_code}' --max-time 5 %q 2>/dev/null || echo fail", url))
		if err == nil && out != "fail" && strings.HasPrefix(out, "2") {
			return nil
		}
		if i < retries-1 {
			time.Sleep(interval)
		}
	}
	return fmt.Errorf("health check %s not OK after %d tries", url, retries)
}

// WaitTCP polls a TCP port on the server.
func WaitTCP(c Client, host string, port int, interval time.Duration, retries int) error {
	for i := 0; i < retries; i++ {
		out, err := c.Output(fmt.Sprintf("nc -z -w 2 %s %d && echo ok || echo fail", host, port))
		if err == nil && out == "ok" {
			return nil
		}
		if i < retries-1 {
			time.Sleep(interval)
		}
	}
	return fmt.Errorf("port %d not open after %d tries", port, retries)
}

func ParseInterval(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Second
	}
	return d
}
