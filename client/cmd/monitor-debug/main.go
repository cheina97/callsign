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
	cameraOn, micOn := false, false

	err := mediaDevices.StartCameraListener(func(isOn bool) {
		log.Printf("camera: %v -> %v, mic: %v", cameraOn, isOn, micOn)
		cameraOn = isOn
	})
	if err != nil {
		log.Fatalf("camera listener: %v", err)
	}
	err = mediaDevices.StartMicrophoneListener(func(isOn bool) {
		log.Printf("camera: %v, mic: %v -> %v", cameraOn, micOn, isOn)
		micOn = isOn
	})
	if err != nil {
		log.Fatalf("microphone listener: %v", err)
	}

	log.Print("listening for camera/microphone events - toggle them to see changes")

	// CoreMediaIO/CoreAudio deliver listener callbacks through the main
	// thread's run loop, which a Go program does not run by default.
	camera.RunMainRunLoop()
}
