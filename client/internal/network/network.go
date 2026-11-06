// Package network detects the Wi-Fi network the Mac is connected to, so the
// client only talks to the ESP32 when it is on the same network.
package network

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

// SSID returns the SSID of the Wi-Fi network the Mac is connected to.
func SSID() (string, error) {
	// macOS 14+ removed SSID access from `airport -I`; system_profiler is the
	// reliable way to get it without Location Services permission.
	out, err := exec.Command("system_profiler", "SPAirPortDataType").Output()
	if err != nil {
		return "", fmt.Errorf("system_profiler: %w", err)
	}
	re := regexp.MustCompile(`(?m)^\s*Current Network Information:\s*\n\s*([^\n:]+):`)
	m := re.FindSubmatch(out)
	if m == nil {
		return "", fmt.Errorf("no current Wi-Fi network found (Wi-Fi off or not connected)")
	}
	return strings.TrimSpace(string(m[1])), nil
}

// OnNetwork reports whether the Mac is connected to the given SSID.
func OnNetwork(expectedSSID string) (bool, error) {
	ssid, err := SSID()
	if err != nil {
		return false, err
	}
	return ssid == expectedSSID, nil
}
