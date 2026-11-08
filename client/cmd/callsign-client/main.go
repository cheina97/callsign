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
//   - A keep-alive ticker re-sends the current state every -keepalive
//     interval (default 4m), so the ESP32's auto-off watchdog only fires
//     when the client is actually gone (stopped, asleep, off-network).
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
	keepAlive      = flag.Duration("keepalive", 4*time.Minute, "Re-send the current state at this interval so the display stays on while the client runs; 0 disables (env: KEEPALIVE_INTERVAL)")
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

	t.send(state)
}

// send reports the state to the display, honoring the Wi-Fi check, and
// records it as lastSent on success.
func (t *tracker) send(state display.State) {
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
	t.lastSent = state
	t.mu.Unlock()
}

// keepAliveLoop re-sends the current state on every tick, so the ESP32's
// auto-off watchdog only fires when the client is actually gone (stopped,
// asleep, off-network) instead of whenever the state is simply unchanged.
// It re-queries the devices rather than reusing lastSent, so it also makes
// progress when no send has succeeded yet (e.g. the Mac just joined the
// right network).
func (t *tracker) keepAliveLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		cameraOn, micOn := monitor.InUse()
		state := display.StateFree
		if cameraOn || micOn {
			state = display.StateCall
		}
		log.Printf("💓 keep-alive: %s", state)
		t.send(state)
	}
}

func onOff(on bool) string {
	if on {
		return "🟢 on"
	}
	return "⚪ off"
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
		// Fail fast with a clear hint if the SSID can't be read at all
		// (e.g. macOS hides it until `sudo ipconfig sethidewifiinfo 0`).
		if ssid, err := network.SSID(); err != nil {
			log.Printf("⚠️  cannot read the current Wi-Fi SSID: %v", err)
		} else {
			log.Printf("📶 on Wi-Fi %q", ssid)
		}
		log.Printf("listening for camera/microphone events, reporting to %s (only on Wi-Fi %q)",
			*esp32URL, *expectedSSID)
	}

	if *keepAlive > 0 {
		log.Printf("keep-alive: re-sending state every %s", *keepAlive)
		go t.keepAliveLoop(*keepAlive)
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
