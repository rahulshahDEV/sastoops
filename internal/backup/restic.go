package backup

import (
	"fmt"
	"strings"
	"time"
)

// Restic commands run on the server with env sourced from backup.env.

func resticCmd(env bool, args ...string) string {
	prefix := ""
	if env {
		prefix = "set -a; . " + EnvFile + "; set +a; "
	}
	return prefix + "restic " + strings.Join(args, " ")
}

func ResticBackup(c RemoteClient, repo string, paths []string, tags []string) (string, error) {
	args := []string{"-r", repo, "backup"}
	for _, t := range tags {
		args = append(args, "--tag", t)
	}
	args = append(args, paths...)
	out, err := c.Output(resticCmd(true, args...))
	if err != nil {
		return "", fmt.Errorf("restic backup: %s", out)
	}
	return out, nil
}

func ResticForget(c RemoteClient, repo string, j JobSpec) error {
	args := []string{"-r", repo, "forget", "--prune"}
	args = append(args, strings.Split(RetentionFlags(j), " ")...)
	out, err := c.Output(resticCmd(true, args...))
	if err != nil {
		return fmt.Errorf("restic forget: %s", out)
	}
	return nil
}

type Snapshot struct {
	ID      string
	Time    time.Time
	Tags    string
	Summary string
}

func ResticSnapshots(c RemoteClient, repo string) ([]Snapshot, error) {
	out, err := c.Output(resticCmd(true, "-r", repo, "snapshots", "--json"))
	if err != nil {
		return nil, fmt.Errorf("restic snapshots: %s", out)
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return nil, nil
	}
	// minimal JSON parse: extract id, time, tags
	var snaps []Snapshot
	entries := splitJSONArray(out)
	for _, e := range entries {
		var id, tm, tags string
		for _, part := range splitJSONFields(e) {
			k, v, ok := strings.Cut(part, ":")
			if !ok {
				continue
			}
			v = strings.Trim(v, "\" ")
			switch k {
			case `"id"`:
				id = v
			case `"time"`:
				if t, err := time.Parse(time.RFC3339, v); err == nil {
					tm = t.Local().Format("2006-01-02 15:04")
				} else {
					tm = v
				}
			case `"tags"`:
				tags = strings.Trim(v, "[]")
			}
		}
		if id != "" {
			snaps = append(snaps, Snapshot{ID: shortID(id), Time: time.Time{}, Tags: tags, Summary: tm})
		}
	}
	return snaps, nil
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func splitJSONArray(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if s == "" {
		return nil
	}
	var out []string
	depth := 0
	start := 0
	inStr := false
	for i, r := range s {
		switch r {
		case '"':
			inStr = !inStr
		case '{':
			if !inStr {
				depth++
			}
		case '}':
			if !inStr {
				depth--
				if depth == 0 {
					out = append(out, s[start:i+1])
					start = i + 1
				}
			}
		case ',':
			if !inStr && depth == 0 {
				start = i + 1
			}
		}
	}
	return out
}

func splitJSONFields(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	var out []string
	depth := 0
	start := 0
	inStr := false
	for i, r := range s {
		switch r {
		case '"':
			inStr = !inStr
		case '{', '[':
			if !inStr {
				depth++
			}
		case '}', ']':
			if !inStr {
				depth--
			}
		case ',':
			if !inStr && depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func ResticRestore(c RemoteClient, repo, snapshot, dest string) error {
	out, err := c.Output(resticCmd(true, "-r", repo, "restore", snapshot, "--target", dest))
	if err != nil {
		return fmt.Errorf("restic restore: %s", out)
	}
	return nil
}

func ResticVerify(c RemoteClient, repo string) error {
	out, err := c.Output(resticCmd(true, "-r", repo, "check", "--read-data-subset", "5%"))
	if err != nil {
		return fmt.Errorf("restic check: %s", out)
	}
	return nil
}

// DBDump renders the dump command for a database type.
func DBDump(composeDir, dbType, container, user, password, db, dest string) string {
	pass := fmt.Sprintf("PGPASSWORD=%q", password)
	switch dbType {
	case "postgres":
		return fmt.Sprintf("cd %s && docker compose exec -T %s sh -c '%s pg_dump -U %s -d %s' > %s", composeDir, container, pass, user, db, dest)
	case "mariadb", "mysql":
		return fmt.Sprintf("cd %s && docker compose exec -T %s sh -c 'MYSQL_PWD=%q mysqldump -u %s %s' > %s", composeDir, container, password, user, db, dest)
	default:
		return ""
	}
}
