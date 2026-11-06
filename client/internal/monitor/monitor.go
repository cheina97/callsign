// Package monitor detects camera and microphone usage on macOS by querying
// the native CoreMediaIO / CoreAudio device state (via cgo), which reports
// whether any process currently holds a device open. This is precise and
// works for any application (native apps, browsers, Electron), unlike
// unified-log scraping which depends on log message formats that change
// between macOS versions.
package monitor

import (
	"log"

	mediaDevices "github.com/antonfisher/go-media-devices-state"
)

// CameraInUse reports whether any camera in the system is currently on.
func CameraInUse() bool {
	on, err := mediaDevices.IsCameraOn()
	if err != nil {
		log.Printf("camera check: %v", err)
		return false
	}
	return on
}

// MicrophoneInUse reports whether any microphone in the system is currently
// capturing audio.
func MicrophoneInUse() bool {
	on, err := mediaDevices.IsMicrophoneOn()
	if err != nil {
		log.Printf("microphone check: %v", err)
		return false
	}
	return on
}

// InUse reports whether the camera or the microphone is currently in use.
func InUse() (camera, mic bool) {
	return CameraInUse(), MicrophoneInUse()
}
