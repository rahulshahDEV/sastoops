package backup

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/rahulshahDEV/sastoops/internal/ui"
	"gopkg.in/yaml.v3"
)

const (
	EnvFile   = "/var/lib/serverops/secrets/backup.env"
	JobsFile  = "/var/lib/serverops/backup/jobs.yaml"
	RcloneCfg = "/var/lib/serverops/secrets/rclone.conf"
)

type RemoteClient interface {
	Output(command string) (string, error)
	Put(data []byte, remotePath, mode string) error
	ReadFile(path string) (string, error)
}

type Jobs struct {
	Engine string    `yaml:"engine"`
	Remote string    `yaml:"remote"`
	Repo   string    `yaml:"repo,omitempty"`
	Jobs   []JobSpec `yaml:"jobs"`
}

type JobSpec struct {
	Name        string   `yaml:"name"`
	Apps        []string `yaml:"apps"`
	Paths       []string `yaml:"paths"`
	Schedule    string   `yaml:"schedule,omitempty"`
	KeepLast    int      `yaml:"keep_last,omitempty"`
	KeepDaily   int      `yaml:"keep_daily,omitempty"`
	KeepMonthly int      `yaml:"keep_monthly,omitempty"`
}

func Endpoint(provider string) string {
	switch provider {
	case "wasabi":
		return "https://s3.wasabisys.com"
	case "r2":
		return "https://<accountid>.r2.cloudflarestorage.com"
	case "b2":
		return "https://s3.us-west-004.backblazeb2.com"
	default:
		return ""
	}
}

func ProviderRegion(provider string) string {
	switch provider {
	case "wasabi":
		return "us-east-1"
	case "r2":
		return "auto"
	case "b2":
		return "us-west-004"
	default:
		return "us-east-1"
	}
}

func InstallEngine(c RemoteClient, engine string) error {
	switch engine {
	case "restic":
		out, err := c.Output("command -v restic >/dev/null 2>&1 && restic version || echo missing")
		if err == nil && !strings.Contains(out, "missing") {
			ui.Info("restic present: %s", strings.SplitN(out, "\n", 2)[0])
			return nil
		}
		ui.Step("installing restic")
		_, err = c.Output(`if [ "$(id -u)" -eq 0 ]; then S=''; else S='sudo -n'; fi; export DEBIAN_FRONTEND=noninteractive; $S apt-get install -y -qq restic >/dev/null 2>&1 || (curl -fsSL -o /tmp/restic.bz2 https://github.com/restic/restic/releases/download/v0.17.3/restic_0.17.3_linux_amd64.bz2 && bunzip2 -f /tmp/restic.bz2 && $S install -m 755 /tmp/restic /usr/local/bin/restic); restic version`)
		if err != nil {
			return fmt.Errorf("restic install failed: %w", err)
		}
		return nil
	case "rclone":
		out, err := c.Output("command -v rclone >/dev/null 2>&1 && rclone version | head -1 || echo missing")
		if err == nil && !strings.Contains(out, "missing") {
			ui.Info("rclone present: %s", out)
			return nil
		}
		ui.Step("installing rclone")
		_, err = c.Output("curl -fsSL https://rclone.org/install.sh | bash >/dev/null 2>&1; rclone version | head -1")
		if err != nil {
			return fmt.Errorf("rclone install failed: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unknown backup engine %q (restic|rclone)", engine)
}

func InitResticRepo(c RemoteClient, repo string) error {
	out, err := c.Output(fmt.Sprintf("set -a; . %s; set +a; restic -r %q snapshots >/dev/null 2>&1 && echo exists || (restic -r %q init && echo created)", EnvFile, repo, repo))
	if err != nil {
		return fmt.Errorf("restic init: %s", out)
	}
	ui.Ok("restic repository ready: %s", repo)
	return nil
}

// RenderResticEnv: RESTIC_PASSWORD + S3 credentials for the repo.
func RenderResticEnv(keyID, secret, region string) string {
	return fmt.Sprintf(`RESTIC_PASSWORD=%s
AWS_ACCESS_KEY_ID=%s
AWS_SECRET_ACCESS_KEY=%s
AWS_DEFAULT_REGION=%s
`, randHex(24), keyID, secret, region)
}

func RenderRcloneConfig(remoteName, provider, keyID, secret string) string {
	vendor := "Other"
	switch provider {
	case "wasabi":
		vendor = "Wasabi"
	case "r2":
		vendor = "Cloudflare"
	case "b2":
		vendor = "Backblaze B2"
	}
	return fmt.Sprintf(`[%s]
type = s3
provider = %s
access_key_id = %s
secret_access_key = %s
endpoint = %s
`, remoteName, vendor, keyID, secret, Endpoint(provider))
}

func WriteEnv(c RemoteClient, content string) error {
	return c.Put([]byte(content), EnvFile, "0600")
}

func ReadEnv(c RemoteClient) (map[string]string, error) {
	raw, err := c.ReadFile(EnvFile)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out, nil
}

func SaveJobs(c RemoteClient, jobs *Jobs) error {
	data, err := yaml.Marshal(jobs)
	if err != nil {
		return err
	}
	return c.Put(data, JobsFile, "0600")
}

func LoadJobs(c RemoteClient) (*Jobs, error) {
	raw, err := c.ReadFile(JobsFile)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("no backup jobs configured — run: sastoops backup setup")
	}
	var j Jobs
	if err := yaml.Unmarshal([]byte(raw), &j); err != nil {
		return nil, err
	}
	return &j, nil
}

func RetentionFlags(j JobSpec) string {
	flags := []string{"--keep-last 7"}
	if j.KeepLast > 0 {
		flags = []string{fmt.Sprintf("--keep-last %d", j.KeepLast)}
	}
	if j.KeepDaily > 0 {
		flags = append(flags, fmt.Sprintf("--keep-daily %d", j.KeepDaily))
	}
	if j.KeepMonthly > 0 {
		flags = append(flags, fmt.Sprintf("--keep-monthly %d", j.KeepMonthly))
	}
	return strings.Join(flags, " ")
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
