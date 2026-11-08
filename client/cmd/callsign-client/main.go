// callsign-client monitors camera/microphone usage on macOS and reports the
// state to an ESP32 + MAX7219 display over its REST API.
//
// Behavior:
//   - OS-level CoreMediaIO/CoreAudio listeners fire the moment the camera or
//     microphone is opened/closed by any process - no polling involved.
//   - On every change it verifies the Mac is on the expected Wi-Fi network
//     (the one the ESP32 lives on). If not, the update is skipped.
//   - If camera or microphone is in use it sends "call", otherwise "free" -
//     but only when the state changed since the last send.
//   - The ESP32 turns the display off by itself after 5 minutes without an
//     update, so no keep-alive is sent while the state is unchanged.
package main

import (
	"flag"
	"log"
	"sync"
	"time"

	"callsign-client/internal/config"
	"callsign-client/internal/display"
	"callsign-client/internal/network"

	"callsign-client/internal/monitor"

	mediaDevices "github.com/antonfisher/go-media-devices-state"
	"github.com/antonfisher/go-media-devices-state/pkg/camera"
)

var (
	esp32URL       = flag.String("esp32", "http://192.168.1.50", "Base URL of the ESP32 REST API (env: ESP32_URL)")
	expectedSSID   = flag.String("ssid", "YOUR_WIFI_SSID", "Wi-Fi SSID the ESP32 is reachable on (env: WIFI_SSID)")
	requestTimeout = flag.Duration("timeout", 5*time.Second, "HTTP request timeout")
	noWiFiCheck    = flag.Bool("no-wifi-check", false, "Skip the Wi-Fi SSID check, always report state (env: SKIP_WIFI_CHECK=true)")
)

// tracker reports display state changes. The OS listeners only tell us
// "something may have changed" (the library deduplicates per device and can
// drop events when a device query fails), so on every callback we re-query
// the ground truth for both devices and dedupe here via lastSent.
type tracker struct {
	mu       sync.Mutex
	cameraOn bool
	micOn    bool
	seen     bool // whether the first state has been logged
	lastSent display.State
	client   *display.Client
}

func newTracker(client *display.Client) *tracker {
	return &tracker{client: client, lastSent: display.State("")}
}

// update re-reads the camera/microphone state, logs every device change
// (like monitor-debug), and when the aggregated state changed since the
// last successful send, reports it to the display. Called from both
// listener callbacks.
func (t *tracker) update() {
	cameraOn, micOn := monitor.InUse()

	t.mu.Lock()
	if !t.seen || cameraOn != t.cameraOn || micOn != t.micOn {
		log.Printf("📷 %s  🎤 %s", onOff(cameraOn), onOff(micOn))
		t.cameraOn, t.micOn, t.seen = cameraOn, micOn, true
	}
	state := display.StateFree
	if cameraOn || micOn {
		state = display.StateCall
	}
	if state == t.lastSent {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	if !*noWiFiCheck {
		ok, err := network.OnNetwork(*expectedSSID)
		switch {
		case err != nil:
			log.Printf("wifi check: %v", err)
			return
		case !ok:
			ssid, _ := network.SSID()
			log.Printf("on Wi-Fi %q, expected %q - skipping update to %s", ssid, *expectedSSID, state)
			return
		}
	}

	if err := t.client.SetState(state); err != nil {
		log.Printf("failed to send state %q: %v", state, err)
		return
	}

	t.mu.Lock()
	firstSend := t.lastSent == display.State("")
	t.lastSent = state
	t.mu.Unlock()
	// The initial "free" is just the display syncing to the idle state the
	// devices were already in - the "devices:" line above covers it.
	if !firstSend || state != display.StateFree {
		log.Printf("%s %s", stateEmoji(state), state)
	}
}

func onOff(on bool) string {
	if on {
		return "🟢 on"
	}
	return "⚪ off"
}

func stateEmoji(s display.State) string {
	if s == display.StateCall {
		return "🔴"
	}
	return "🟢"
}

func main() {
	config.Load(".env")
	flag.Parse()
	config.ApplyEnv()

	t := newTracker(display.NewClient(*esp32URL, *requestTimeout))

	if *noWiFiCheck {
		log.Printf("listening for camera/microphone events, reporting to %s (Wi-Fi check disabled)",
			*esp32URL)
	} else {
		log.Printf("listening for camera/microphone events, reporting to %s (only on Wi-Fi %q)",
			*esp32URL, *expectedSSID)
	}

	// The callback argument is ignored on purpose: update() re-queries both
	// devices, so a missed/stale notification from one listener can never
	// leave the display in the wrong state.
	onChange := func(bool) { t.update() }
	if err := mediaDevices.StartCameraListener(onChange); err != nil {
		log.Fatalf("camera listener: %v", err)
	}
	if err := mediaDevices.StartMicrophoneListener(onChange); err != nil {
		log.Fatalf("microphone listener: %v", err)
	}

	// CoreMediaIO/CoreAudio deliver listener callbacks through the main
	// thread's run loop, which a Go program does not run by default.
	camera.RunMainRunLoop()
}
