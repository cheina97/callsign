// Package config loads environment variables, optionally from a .env file.
// It has no external dependencies: real environment variables always win
// over values defined in the file.
package config

import (
	"bufio"
	"flag"
	"log"
	"os"
	"strings"
)

// Load reads the .env file at path and sets any variables that are not
// already present in the environment. A missing file is not an error.
func Load(path string) {
	f, err := os.Open(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("config: cannot open %s: %v", path, err)
		}
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Printf("config: error reading %s: %v", path, err)
	}
}

// Get returns the value of the environment variable, or fallback if unset/empty.
func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// flagEnv maps flag names to the environment variable that overrides them.
// Only flags listed here can be overridden via the environment / .env.
var flagEnv = map[string]string{
	"esp32": "ESP32_URL",
	"ssid":  "WIFI_SSID",
	"addr":  "ADDR",
}

// ApplyEnv overrides flag values with the environment variables mapped in
// flagEnv, but only for flags that were not explicitly set on the command
// line. Call after flag.Parse(). Priority: CLI flag > env var / .env > default.
func ApplyEnv() {
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	for flagName, envName := range flagEnv {
		if explicit[flagName] {
			continue
		}
		f := flag.Lookup(flagName)
		if f == nil {
			continue // flag not defined by this binary
		}
		if v := os.Getenv(envName); v != "" {
			if err := f.Value.Set(v); err != nil {
				log.Printf("config: invalid value %q for %s: %v", v, envName, err)
			}
		}
	}
}
