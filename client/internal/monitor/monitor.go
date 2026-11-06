// Package monitor detects camera and microphone usage on macOS by inspecting
// the unified log.
//
// Modern macOS (14+) no longer logs the old VDCAssistant_Power_State /
// HALS_Device messages the previous implementation relied on. Instead:
//
//   - Camera: the com.apple.cameracapture subsystem logs
//     "AVCaptureSession ... _setRunning: ... running -> 1" when an app starts
//     the camera and "running -> 0" when it stops.
//   - Microphone: the com.apple.coreaudio:as_client category logs
//     "setPlayState Started  Input {Device, 0xNN}" / "setPlayState Stopped
//     Input {Device, 0xNN}" when an app starts/stops capturing audio. The
//     0xNN session id lets us pair starts with stops.
//
// The package keeps a running count of active camera/mic sessions. Each call
// to InUse scans only the log entries produced since the previous call and
// applies the deltas, so a device is reported "in use" only between its
// start and stop events.
package monitor

import (
	"log"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// runLogged runs a command and returns its combined output, logging failures.
func runLogged(name string, args ...string) string {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		log.Printf("%s %v: %v (%s)", name, args, err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

var (
	cameraRunningRe = regexp.MustCompile(`running -> ([01])`)
	micPlayStateRe  = regexp.MustCompile(`setPlayState (Started|Stopped)\s+Input(?:/Output)? \{[^,]+, (0x[0-9a-fA-F]+)\}`)
)

// tracker follows camera/mic start/stop events across polls.
type tracker struct {
	mu         sync.Mutex
	lastCheck  time.Time
	cameraOpen int            // number of unmatched "running -> 1"
	micOpen    map[string]int // active mic sessions by session id
}

var defaultTracker = &tracker{micOpen: map[string]int{}}

// scanWindow returns the --last argument for the next log query: everything
// since the previous check (plus a small overlap to avoid missing entries at
// the boundary), or a fixed window for the very first check.
func (t *tracker) scanWindow() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastCheck.IsZero() {
		return "2m"
	}
	d := time.Since(t.lastCheck) + 5*time.Second
	return d.Round(time.Second).String()
}

// applyCamera folds camera start/stop events into the running count.
func (t *tracker) applyCamera(out string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, line := range strings.Split(out, "\n") {
		m := cameraRunningRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == "1" {
			t.cameraOpen++
		} else if t.cameraOpen > 0 {
			t.cameraOpen--
		}
	}
}

// applyMic folds mic start/stop events into the session map.
func (t *tracker) applyMic(out string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, line := range strings.Split(out, "\n") {
		m := micPlayStateRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if m[1] == "Started" {
			t.micOpen[m[2]]++
		} else if t.micOpen[m[2]] > 0 {
			t.micOpen[m[2]]--
		}
	}
}

// state returns the current camera/mic usage.
func (t *tracker) state() (camera, mic bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	micActive := false
	for _, n := range t.micOpen {
		if n > 0 {
			micActive = true
			break
		}
	}
	return t.cameraOpen > 0, micActive
}

// InUse reports whether the camera or the microphone is currently in use.
// The first call scans the last 2 minutes of logs; subsequent calls scan only
// what happened since the previous call.
func InUse() (camera, mic bool) {
	window := defaultTracker.scanWindow()

	cameraOut := runLogged("log", "show", "--last", window, "--style", "compact",
		"--predicate", `subsystem == "com.apple.cameracapture" AND eventMessage CONTAINS "running -> "`)
	micOut := runLogged("log", "show", "--last", window, "--style", "compact",
		"--predicate", `subsystem == "com.apple.coreaudio" AND eventMessage CONTAINS "setPlayState" AND eventMessage CONTAINS "Input"`)

	defaultTracker.applyCamera(cameraOut)
	defaultTracker.applyMic(micOut)

	defaultTracker.mu.Lock()
	defaultTracker.lastCheck = time.Now()
	defaultTracker.mu.Unlock()

	return defaultTracker.state()
}
