// Device-dependent end-to-end tests: they exercise the real camera and
// microphone detection (internal/monitor) by starting a helper that streams
// from the actual hardware. They are skipped automatically on machines
// without a camera/microphone (e.g. CI runners), so `make test-e2e` is safe
// to run anywhere.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"callsign-client/internal/monitor"
)

// avtriggerBin compiles the Swift capture helper once per test run and
// returns its path. It starts a real AVCaptureSession pulling samples from
// the camera or microphone for a given number of seconds.
func avtriggerBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(os.TempDir(), "callsign-avtrigger-e2e")
	src := filepath.Join("avtrigger", "main.swift")
	build := exec.Command("swiftc", "-O", "-o", bin, src)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build avtrigger: %v\n%s", err, out)
	}
	return bin
}

// stream starts `avtrigger <mode> <seconds>` in the background and returns a
// stop function that waits for the helper to exit.
func stream(t *testing.T, bin, mode string, seconds int) (stop func()) {
	t.Helper()
	cmd := exec.Command(bin, mode, fmt.Sprintf("%d", seconds))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start avtrigger %s: %v", mode, err)
	}
	return func() { cmd.Wait() }
}

// waitFor polls cond until it returns true or the deadline passes.
func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s: condition not met within %s", what, timeout)
}

// TestCameraDetectionRequiresHardware streams from the real camera for a few
// seconds and verifies that monitor.CameraInUse reports it. Skipped when no
// camera is present.
func TestCameraDetectionRequiresHardware(t *testing.T) {
	if monitor.CameraInUse() {
		t.Skip("camera already in use by another application")
	}
	bin := avtriggerBin(t)

	// Probe: if the helper cannot start a stream there is no usable camera.
	probe := exec.Command(bin, "camera", "1")
	if err := probe.Run(); err != nil {
		t.Skipf("no usable camera on this machine: %v", err)
	}

	stop := stream(t, bin, "camera", 10)
	defer stop()

	waitFor(t, "camera detected as in use", 8*time.Second, monitor.CameraInUse)

	// Let the stream end and verify the state flips back.
	stop()
	waitFor(t, "camera detected as free after stream ended", 8*time.Second,
		func() bool { return !monitor.CameraInUse() })
}

// TestMicrophoneDetectionRequiresHardware streams from the real microphone
// for a few seconds and verifies that monitor.MicrophoneInUse reports it.
// Skipped when no microphone is present.
func TestMicrophoneDetectionRequiresHardware(t *testing.T) {
	if monitor.MicrophoneInUse() {
		t.Skip("microphone already in use by another application")
	}
	bin := avtriggerBin(t)

	probe := exec.Command(bin, "mic", "1")
	if err := probe.Run(); err != nil {
		t.Skipf("no usable microphone on this machine: %v", err)
	}

	stop := stream(t, bin, "mic", 10)
	defer stop()

	waitFor(t, "microphone detected as in use", 8*time.Second, monitor.MicrophoneInUse)

	stop()
	waitFor(t, "microphone detected as free after stream ended", 8*time.Second,
		func() bool { return !monitor.MicrophoneInUse() })
}
