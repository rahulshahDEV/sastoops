package backup

import (
	"fmt"
	"strings"
)

// Rclone engine: raw sync to any rclone remote (S3-compatible, Drive, etc).

func rcloneCfg(args ...string) string {
	return "rclone --config " + RcloneCfg + " " + strings.Join(args, " ")
}

func RcloneTest(c RemoteClient, remote string) error {
	out, err := c.Output(rcloneCfg("lsd", remote+":") + " 2>&1 | head -5")
	if err != nil {
		return fmt.Errorf("rclone connectivity failed: %s", out)
	}
	return nil
}

func RcloneBackup(c RemoteClient, remote, prefix string, paths []string) (string, error) {
	args := []string{"sync", "--progress", "--stats-one-line"}
	for _, p := range paths {
		args = append(args, p, fmt.Sprintf("%s:%s/", remote, prefix))
	}
	out, err := c.Output(rcloneCfg(args...) + " 2>&1 | tail -3")
	if err != nil {
		return "", fmt.Errorf("rclone sync: %s", out)
	}
	return out, nil
}

func RcloneList(c RemoteClient, remote, prefix string) (string, error) {
	out, err := c.Output(rcloneCfg("lsf", "-R", "--files-only", fmt.Sprintf("%s:%s/", remote, prefix)) + " 2>&1 | head -50")
	if err != nil {
		return "", fmt.Errorf("rclone lsf: %s", out)
	}
	return out, nil
}

func RcloneRestore(c RemoteClient, remote, prefix, dest string) error {
	out, err := c.Output(rcloneCfg("copy", fmt.Sprintf("%s:%s/", remote, prefix), dest) + " 2>&1 | tail -3")
	if err != nil {
		return fmt.Errorf("rclone restore: %s", out)
	}
	return nil
}

func RcloneVerify(c RemoteClient, remote, prefix, local string) error {
	out, err := c.Output(rcloneCfg("check", fmt.Sprintf("%s:%s/", remote, prefix), local) + " 2>&1 | tail -5")
	if err != nil {
		return fmt.Errorf("rclone check: %s", out)
	}
	if !strings.Contains(out, "0 differences") {
		return fmt.Errorf("rclone check found differences: %s", out)
	}
	return nil
}
