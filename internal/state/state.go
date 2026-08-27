package state

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

const (
	Dir        = "/var/lib/serverops"
	StateFile  = Dir + "/state.json"
	SchemaVer  = 1
	EnvDir     = Dir + "/env"
	ComposeDir = Dir + "/compose"
	SecretDir  = Dir + "/secrets"
	DumpDir    = Dir + "/backup/dumps"
)

type ServerState struct {
	SchemaVersion int                  `json:"schema_version"`
	Hostname      string               `json:"hostname,omitempty"`
	Modules       map[string]ModuleTag `json:"modules"`
	Recipes       map[string]string    `json:"recipes"`
	Apps          map[string]*AppState `json:"apps"`
	Backups       *BackupState         `json:"backups,omitempty"`
	UpdatedAt     string               `json:"updated_at,omitempty"`
}

type ModuleTag struct {
	Version string `json:"version"`
	At      string `json:"at"`
}

type AppState struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Domain      string `json:"domain,omitempty"`
	InstalledAt string `json:"installed_at,omitempty"`
}

type BackupState struct {
	Engine     string `json:"engine"`
	Remote     string `json:"remote"`
	Repo       string `json:"repo"`
	LastRun    string `json:"last_run,omitempty"`
	LastStatus string `json:"last_status,omitempty"`
}

func New() *ServerState {
	return &ServerState{
		SchemaVersion: SchemaVer,
		Modules:       map[string]ModuleTag{},
		Recipes:       map[string]string{},
		Apps:          map[string]*AppState{},
	}
}

func (s *ServerState) ModuleApplied(id, version string) bool {
	t, ok := s.Modules[id]
	return ok && t.Version == version
}

func (s *ServerState) MarkModule(id, version string) {
	if s.Modules == nil {
		s.Modules = map[string]ModuleTag{}
	}
	s.Modules[id] = ModuleTag{Version: version, At: time.Now().UTC().Format(time.RFC3339)}
}

// SaveJSON writes state atomically to a remote path via a writer callback.
func (s *ServerState) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return data, nil
}

func Parse(data []byte) (*ServerState, error) {
	s := New()
	if err := json.Unmarshal(data, s); err != nil {
		return nil, fmt.Errorf("invalid state: %w", err)
	}
	if s.SchemaVersion > SchemaVer {
		return nil, fmt.Errorf("state schema v%d is newer than supported v%d", s.SchemaVersion, SchemaVer)
	}
	return s, nil
}

func LocalCacheDir() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		return os.TempDir() + "/sastoops"
	}
	return dir + "/sastoops"
}

func (s *ServerState) WriteLocalCache(server string) error {
	dir := LocalCacheDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, _ := s.Marshal()
	return os.WriteFile(dir+"/"+server+".json", data, 0o600)
}

func ReadLocalCache(server string) (*ServerState, error) {
	data, err := os.ReadFile(LocalCacheDir() + "/" + server + ".json")
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// RemoteLoad fetches state from a server (empty state if absent).
func RemoteLoad(client interface {
	ReadFile(path string) (string, error)
}) (*ServerState, error) {
	raw, err := client.ReadFile(StateFile)
	if err != nil {
		return nil, err
	}
	if raw == "" {
		return New(), nil
	}
	return Parse([]byte(raw))
}

// RemoteSave writes state to a server atomically.
func RemoteSave(client interface {
	Put(data []byte, remotePath, mode string) error
}, st *ServerState) error {
	st.SchemaVersion = SchemaVer
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := st.Marshal()
	if err != nil {
		return err
	}
	return client.Put(data, StateFile, "0644")
}
