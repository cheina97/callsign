// monitor-debug polls camera/microphone usage and prints a line every time
// the detected state changes. Useful for testing detection without any
// server or display involved.
package main

import (
	"flag"
	"log"
	"time"

	"callsign-client/internal/monitor"
)

var interval = flag.Duration("interval", 2*time.Second, "How often to check camera/microphone state")

func main() {
	flag.Parse()
	log.Printf("polling camera/microphone every %s - toggle them to see changes", *interval)

	lastCamera, lastMic := false, false
	first := true
	for {
		camera, mic := monitor.InUse()
		log.Printf("check: camera: %v, mic: %v", camera, mic)
		if first || camera != lastCamera || mic != lastMic {
			log.Printf("camera: %v -> %v, mic: %v -> %v", lastCamera, camera, lastMic, mic)
			lastCamera, lastMic = camera, mic
			first = false
		}
		time.Sleep(*interval)
	}
}
