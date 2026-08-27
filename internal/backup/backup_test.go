package backup

import (
	"strings"
	"testing"
)

type fakeClient struct {
	outputs map[string]string
	puts    [][]byte
}

func (f *fakeClient) Output(cmd string) (string, error) {
	for k, v := range f.outputs {
		if strings.Contains(cmd, k) {
			return v, nil
		}
	}
	return "", nil
}
func (f *fakeClient) Put(data []byte, path, mode string) error {
	f.puts = append(f.puts, data)
	return nil
}
func (f *fakeClient) ReadFile(path string) (string, error) { return "", nil }
func (f *fakeClient) Exists(path string) (bool, error)     { return false, nil }

func TestEndpoints(t *testing.T) {
	cases := map[string]string{
		"wasabi": "https://s3.wasabisys.com",
		"r2":     "https://<accountid>.r2.cloudflarestorage.com",
		"b2":     "https://s3.us-west-004.backblazeb2.com",
		"other":  "",
	}
	for provider, want := range cases {
		if got := Endpoint(provider); got != want {
			t.Errorf("%s: got %s want %s", provider, got, want)
		}
	}
}

func TestRetentionFlags(t *testing.T) {
	j := JobSpec{KeepLast: 3, KeepDaily: 7, KeepMonthly: 2}
	out := RetentionFlags(j)
	if !strings.Contains(out, "--keep-last 3") || !strings.Contains(out, "--keep-daily 7") || !strings.Contains(out, "--keep-monthly 2") {
		t.Errorf("bad retention flags: %s", out)
	}
}

func TestRenderRcloneConfig(t *testing.T) {
	cfg := RenderRcloneConfig("wasabi", "wasabi", "keyid", "secret")
	for _, want := range []string{"[wasabi]", "provider = Wasabi", "access_key_id = keyid", "endpoint = https://s3.wasabisys.com"} {
		if !strings.Contains(cfg, want) {
			t.Errorf("rclone config missing %q:\n%s", want, cfg)
		}
	}
}

func TestResticSnapshotsParse(t *testing.T) {
	json := `[
  {"id":"a1b2c3d4e5f6","time":"2026-08-27T10:00:00Z","tags":["daily"],"paths":["/var/lib/serverops"]},
  {"id":"f6e5d4c3b2a1","time":"2026-08-26T10:00:00Z","tags":null}
]`
	fc := &fakeClient{outputs: map[string]string{"snapshots": json}}
	snaps, err := ResticSnapshots(fc, "s3:repo")
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}
	if snaps[0].ID != "a1b2c3d4" {
		t.Errorf("short id: %s", snaps[0].ID)
	}
	if !strings.Contains(snaps[0].Summary, "2026-08-27") {
		t.Errorf("date: %s", snaps[0].Summary)
	}
}

func TestDBDump(t *testing.T) {
	cmd := DBDump("/var/lib/serverops/compose/n8n", "postgres", "n8n-db", "n8n", "pw123", "n8n", "/tmp/n8n.sql")
	for _, want := range []string{"pg_dump", "-U n8n", "-d n8n", "/tmp/n8n.sql", "compose exec -T n8n-db"} {
		if !strings.Contains(cmd, want) {
			t.Errorf("pg dump missing %q: %s", want, cmd)
		}
	}
	cmd2 := DBDump("/x", "mariadb", "mc", "u", "p", "db", "/d.sql")
	if !strings.Contains(cmd2, "mysqldump") {
		t.Errorf("mariadb dump wrong: %s", cmd2)
	}
}

func TestRenderResticEnv(t *testing.T) {
	env := RenderResticEnv("k", "s", "us-east-1")
	if !strings.Contains(env, "RESTIC_PASSWORD=") || !strings.Contains(env, "AWS_ACCESS_KEY_ID=k") {
		t.Errorf("env incomplete: %s", env)
	}
}
