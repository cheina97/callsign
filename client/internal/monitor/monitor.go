// Package monitor detects camera and microphone usage on macOS by inspecting
// the unified log.
package monitor

import (
	"log"
	"os/exec"
	"strings"
)

// runLogged runs a command and returns its combined output, logging failures.
func runLogged(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		log.Printf("%s %v: %v (%s)", name, args, err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

// CameraInUse reports whether any process holds a VDC (FaceTime) camera open.
func CameraInUse() bool {
	out := runLogged("log", "show", "--last", "2m", "--style", "compact",
		"--predicate", `subsystem == "com.apple.cmio" AND eventMessage CONTAINS "VDCAssistant"`)
	return strings.Contains(out, "VDCAssistant_Power_State = On")
}

// MicrophoneInUse reports whether any process holds an audio input device open.
func MicrophoneInUse() bool {
	out := runLogged("log", "show", "--last", "2m", "--style", "compact",
		"--predicate", `subsystem == "com.apple.coreaudio" AND eventMessage CONTAINS "HALS_Device"`)
	return strings.Contains(out, "isInput = 1") && strings.Contains(out, "isRunning = 1")
}

// InUse reports whether the camera or the microphone is currently in use.
func InUse() (camera, mic bool) {
	camera = CameraInUse()
	mic = MicrophoneInUse()
	return camera, mic
}
