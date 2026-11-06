// monitor-debug listens for camera/microphone state changes via OS-level
// event listeners and prints a line every time either changes. Useful for
// testing detection without any server or display involved.
package main

import (
	"log"

	mediaDevices "github.com/antonfisher/go-media-devices-state"
	"github.com/antonfisher/go-media-devices-state/pkg/camera"
)

func main() {
	status := func(icon bool) string {
		if icon {
			return "🟢 on"
		}
		return "⚪ off"
	}
	cameraOn, micOn := false, false
	printStatus := func() {
		log.Printf("📷 %s  🎤 %s", status(cameraOn), status(micOn))
	}

	err := mediaDevices.StartCameraListener(func(isOn bool) {
		cameraOn = isOn
		printStatus()
	})
	if err != nil {
		log.Fatalf("camera listener: %v", err)
	}
	err = mediaDevices.StartMicrophoneListener(func(isOn bool) {
		micOn = isOn
		printStatus()
	})
	if err != nil {
		log.Fatalf("microphone listener: %v", err)
	}

	log.Print("listening for camera/microphone events - toggle them to see changes")

	// CoreMediaIO/CoreAudio deliver listener callbacks through the main
	// thread's run loop, which a Go program does not run by default.
	camera.RunMainRunLoop()
}
