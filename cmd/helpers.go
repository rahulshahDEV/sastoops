package cmd

import (
	"net"
	"os"
	"time"

	"github.com/rahulshahDEV/sastoops/internal/app"
	"github.com/rahulshahDEV/sastoops/internal/monitor"
	"github.com/rahulshahDEV/sastoops/internal/recipe"
	"github.com/rahulshahDEV/sastoops/internal/ssh"
	"github.com/rahulshahDEV/sastoops/internal/state"
	"github.com/rahulshahDEV/sastoops/internal/ui"
)

type sshClient = ssh.Client

type monitorStats = monitor.Stats

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return dir + "/sastoops", nil
}

func collectStats(client *ssh.Client) (*monitorStats, error) {
	return monitor.Collect(client)
}

func netDial(ip string) (net.Conn, error) {
	return net.DialTimeout("tcp", ip+":22", 3*time.Second)
}

func lookupEnv(key string) string {
	return os.Getenv(key)
}

// loadApp loads an app definition (embedded + user overlay).
func loadApp(name string) (*app.App, error) {
	dir := ""
	if d, err := configDir(); err == nil {
		dir = d + "/apps"
	}
	return app.Load(name, dir)
}

// loadRemoteState reads state from the server (fresh) and mirrors locally.
func loadRemoteState(client *ssh.Client, serverName string) (*state.ServerState, error) {
	st, err := state.RemoteLoad(client)
	if err != nil {
		return nil, err
	}
	_ = st.WriteLocalCache(serverName)
	return st, nil
}

func saveRemoteState(client *ssh.Client, st *state.ServerState) error {
	return state.RemoteSave(client, st)
}

// runServerSetup applies a recipe to a server.
func runServerSetup(name, recipeName string) error {
	client, _, err := dial(name)
	if err != nil {
		return err
	}
	defer client.Close()
	return applyRecipe(client, name, recipeName, map[string]string{})
}

func applyRecipe(client *sshClient, serverName, recipeName string, overrides map[string]string) error {
	r, err := recipe.Load(recipeName)
	if err != nil {
		return err
	}
	st, err := loadRemoteState(client, serverName)
	if err != nil {
		return err
	}
	params := r.ResolveParams(overrides)
	engine := recipe.NewEngine(client, st, G.Check)
	ui.Section("recipe: %s", r.Name)
	applied, err := engine.Apply(r, params)
	if err != nil {
		return err
	}
	if G.Check {
		if len(applied) == 0 {
			ui.Ok("nothing to change — recipe already applied")
		} else {
			ui.Info("%d step(s) would run: %v", len(applied), applied)
		}
		return nil
	}
	if len(applied) > 0 {
		r2 := st.Recipes
		if r2 == nil {
			r2 = map[string]string{}
		}
		r2[recipeName] = r.Version
		st.Recipes = r2
		if err := saveRemoteState(client, st); err != nil {
			return err
		}
		ui.Ok("recipe %s applied", r.Name)
	} else {
		ui.Info("recipe %s already up to date", r.Name)
	}
	return nil
}
