// Package test holds all sastoops tests: unit tests against internal
// packages and black-box CLI tests against the real binary.
package test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "sastoops-test-*")
	if err != nil {
		panic(err)
	}
	binPath = filepath.Join(dir, "sastoops")
	cmd := exec.Command("go", "build", "-o", binPath, "..")
	cmd.Dir = "."
	out, err := cmd.CombinedOutput()
	if err != nil {
		panic("build failed: " + string(out))
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// freshHome creates an isolated HOME for one test (isolates config, ssh keys).
func freshHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	os.MkdirAll(home+"/.ssh", 0o700)
	return home
}

// env builds a clean environment with an isolated HOME.
func env(home string) []string {
	return append([]string{
		"HOME=" + home,
		"XDG_CONFIG_HOME=" + home + "/config",
	}, "PATH="+os.Getenv("PATH"))
}

// run executes the built binary; returns combined output and exit code.
func run(home string, args ...string) (string, int) {
	cmd := exec.Command(binPath, args...)
	cmd.Env = env(home)
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return string(out), code
}
