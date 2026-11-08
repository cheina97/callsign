package config

import (
	"os"
	"testing"
)

func TestLoadAndApplyEnv(t *testing.T) {
	// The Makefile exports the repo-root .env into the test environment;
	// unset the keys under test so Load() can set them from the temp file.
	t.Setenv("ESP32_URL", "")
	t.Setenv("WIFI_SSID", "")
	os.Unsetenv("ESP32_URL")
	os.Unsetenv("WIFI_SSID")

	dir := t.TempDir()
	envFile := dir + "/.env"
	if err := os.WriteFile(envFile, []byte("ESP32_URL=http://localhost:18080\nWIFI_SSID=TestNet\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	Load(envFile)
	if got := os.Getenv("ESP32_URL"); got != "http://localhost:18080" {
		t.Errorf("ESP32_URL = %q", got)
	}
	if got := os.Getenv("WIFI_SSID"); got != "TestNet" {
		t.Errorf("WIFI_SSID = %q", got)
	}
}
