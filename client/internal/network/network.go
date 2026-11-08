// Package network detects the Wi-Fi network the Mac is connected to, so the
// client only talks to the ESP32 when it is on the same network.
package network

import (
	"fmt"
	"os/exec"
	"strings"
)

// redactedSSID is what macOS reports instead of the real SSID when Wi-Fi
// info is hidden (the default until `sudo ipconfig sethidewifiinfo 0` is run).
const redactedSSID = "<redacted>"

// SSID returns the SSID of the Wi-Fi network the Mac is connected to.
//
// It uses `ipconfig getsummary`, which is near-instant, unlike
// `system_profiler SPAirPortDataType` which takes several seconds. Note that
// macOS hides the SSID unless the user has run
// `sudo ipconfig sethidewifiinfo 0`; in that case an error is returned.
func SSID() (string, error) {
	out, err := exec.Command("ipconfig", "getsummary", "en0").Output()
	if err != nil {
		return "", fmt.Errorf("ipconfig getsummary: %w", err)
	}
	// The output contains both "BSSID : ..." and "SSID : ..." lines; the
	// SSID of the current network is the last one.
	var ssid string
	for _, line := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "SSID : "); ok {
			ssid = strings.TrimSpace(v)
		}
	}
	if ssid == "" {
		return "", fmt.Errorf("no current Wi-Fi network found (Wi-Fi off or not connected)")
	}
	if ssid == redactedSSID {
		return "", fmt.Errorf("macOS is hiding the Wi-Fi SSID - run `sudo ipconfig sethidewifiinfo 0` to allow the client to see it")
	}
	return ssid, nil
}

// OnNetwork reports whether the Mac is connected to the given SSID.
func OnNetwork(expectedSSID string) (bool, error) {
	ssid, err := SSID()
	if err != nil {
		return false, err
	}
	return ssid == expectedSSID, nil
}
