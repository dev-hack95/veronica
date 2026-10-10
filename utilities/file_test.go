package utilities

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// homeWithConfig points HOME at a temp dir and optionally seeds an existing
// ~/.veronica/model.yaml, mirroring how InitModel calls CheckFileExist.
func homeWithConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if body != "" {
		vdir := filepath.Join(dir, ".veronica")
		if err := os.MkdirAll(vdir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(vdir, "model.yaml"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCheckFileExist_LegacyTempreatureKey(t *testing.T) {
	homeWithConfig(t, "model: m\nurl: u\nstream: true\ntempreature: 0.55\ntopp: 0.7\nmaxtokens: 100\n")
	cfg, err := CheckFileExist("/.veronica/model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature != 0.55 {
		t.Fatalf("legacy key: got %v want 0.55", cfg.Temperature)
	}
}

func TestCheckFileExist_NewTemperatureKey(t *testing.T) {
	homeWithConfig(t, "model: m\nurl: u\nstream: true\ntemperature: 0.42\ntopp: 0.7\nmaxtokens: 100\n")
	cfg, err := CheckFileExist("/.veronica/model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature != 0.42 {
		t.Fatalf("new key: got %v want 0.42", cfg.Temperature)
	}
}

func TestCheckFileExist_BothKeysPreferNew(t *testing.T) {
	homeWithConfig(t, "temperature: 0.2\ntempreature: 0.9\n")
	cfg, err := CheckFileExist("/.veronica/model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature != 0.2 {
		t.Fatalf("both keys: got %v want 0.2", cfg.Temperature)
	}
}

func TestCheckFileExist_FreshDefaultUsesCleanKey(t *testing.T) {
	dir := homeWithConfig(t, "")
	cfg, err := CheckFileExist("/.veronica/model.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Temperature != 0.8 {
		t.Fatalf("fresh default: got %v want 0.8", cfg.Temperature)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".veronica", "model.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "temperature: 0.8") {
		t.Fatalf("fresh default should contain 'temperature: 0.8', got:\n%s", s)
	}
	if strings.Contains(s, "tempreature") {
		t.Fatalf("fresh default must not contain 'tempreature', got:\n%s", s)
	}
}
