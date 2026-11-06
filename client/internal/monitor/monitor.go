// Package monitor detects camera and microphone usage on macOS by inspecting
// the unified log.
//
// Modern macOS (14+) no longer logs the old VDCAssistant_Power_State /
// HALS_Device messages the previous implementation relied on. Instead:
//
//   - Camera: the com.apple.cameracapture subsystem logs
//     "AVCaptureSession ... _setRunning: ... running -> 1" when an app starts
//     the camera and "running -> 0" when it stops.
//   - Microphone: two log shapes are tracked:
//     1. com.apple.coreaudio:as_client logs "setPlayState Started  Input
//     {Device, 0xNN}" / "setPlayState Stopped Input {Device, 0xNN}" (used
//     by native apps like Discord). The 0xNN session id pairs starts with
//     stops. corespeechd (Siri's always-on voice service) is ignored: it
//     holds an input session open permanently and is not a call.
//     2. audiomxd (com.apple.coreaudio:as_server) logs
//     "update_running_state" JSON blobs with "input_running":true/false
//     per audio session (used by browsers, e.g. Chrome/Google Meet). The
//     session "ID" pairs the updates.
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
	micRunningRe    = regexp.MustCompile(`"session":\{"ID":"(0x[0-9a-fA-F]+)".*"input_running":(true|false)`)
)

// tracker follows camera/mic start/stop events across polls.
type tracker struct {
	mu         sync.Mutex
	lastCheck  time.Time
	cameraOpen int             // number of unmatched "running -> 1"
	micOpen    map[string]bool // active mic sessions by session id
}

var defaultTracker = &tracker{micOpen: map[string]bool{}}

// scanWindow returns the --last argument for the next log query: everything
// since the previous check (plus a small overlap to avoid missing entries at
// the boundary), or a fixed window for the very first check.
func (t *tracker) scanWindow() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastCheck.IsZero() {
		// First check: scan far enough back to catch sessions that started
		// before the program was launched (e.g. an ongoing call).
		return "10m"
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
		// corespeechd (Siri) holds an input session open permanently.
		if strings.Contains(line, "corespeechd") {
			continue
		}
		m := micPlayStateRe.FindStringSubmatch(line)
		if m != nil {
			if m[1] == "Started" {
				t.micOpen[m[2]] = true
			} else {
				delete(t.micOpen, m[2])
			}
			continue
		}
		// audiomxd update_running_state (browsers, e.g. Chrome/Meet).
		if m := micRunningRe.FindStringSubmatch(line); m != nil {
			if m[2] == "true" {
				t.micOpen[m[1]] = true
			} else {
				delete(t.micOpen, m[1])
			}
		}
	}
}

// state returns the current camera/mic usage.
func (t *tracker) state() (camera, mic bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.cameraOpen > 0, len(t.micOpen) > 0
}

// InUse reports whether the camera or the microphone is currently in use.
// The first call scans the last 10 minutes of logs (to catch sessions that
// started before the program was launched); subsequent calls scan only what
// happened since the previous call.
func InUse() (camera, mic bool) {
	window := defaultTracker.scanWindow()

	cameraOut := runLogged("log", "show", "--last", window, "--style", "compact",
		"--predicate", `subsystem == "com.apple.cameracapture" AND eventMessage CONTAINS "running -> "`)
	micOut := runLogged("log", "show", "--last", window, "--style", "compact",
		"--predicate", `subsystem == "com.apple.coreaudio" AND (eventMessage CONTAINS "setPlayState" OR eventMessage CONTAINS "update_running_state") AND eventMessage CONTAINS[c] "input"`)

	defaultTracker.applyCamera(cameraOut)
	defaultTracker.applyMic(micOut)

	defaultTracker.mu.Lock()
	defaultTracker.lastCheck = time.Now()
	defaultTracker.mu.Unlock()

	return defaultTracker.state()
}
