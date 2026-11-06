// Device-dependent end-to-end tests: they exercise the real camera and
// microphone detection (internal/monitor) by capturing from the actual
// hardware with ffmpeg. They are skipped automatically when ffmpeg is not
// installed or no camera/microphone is available (e.g. CI runners), so
// `make test-e2e` is safe to run anywhere.
package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"callsign-client/internal/monitor"
)

// ffmpegArgs returns the ffmpeg arguments to capture from the given device
// kind ("camera" or "mic") for the given number of seconds, discarding all
// output. The camera is AVFoundation video device 0, the microphone is the
// default audio device (":0").
func ffmpegArgs(kind string, seconds int) []string {
	args := []string{
		"-hide_banner", "-loglevel", "error",
		"-f", "avfoundation",
	}
	input := ":0" // default audio input
	if kind == "camera" {
		input = "0" // first video device
		// The default framerate (29.97) is not supported by some cameras.
		args = append(args, "-framerate", "30")
	}
	return append(args,
		"-i", input,
		"-t", fmt.Sprintf("%d", seconds),
		"-f", "null", "-",
	)
}

// ffmpegPath skips the test if ffmpeg is not installed.
func ffmpegPath(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed (brew install ffmpeg)")
	}
	return path
}

// probeDevice skips the test if a short capture from the device fails,
// which is the case on machines without the hardware (CI runners, VMs).
func probeDevice(t *testing.T, ffmpeg, kind string) {
	t.Helper()
	cmd := exec.Command(ffmpeg, ffmpegArgs(kind, 1)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("no usable %s on this machine: %v (%s)", kind, err, out)
	}
}

// stream starts an ffmpeg capture in the background and returns a stop
// function that waits for it to exit.
func stream(t *testing.T, ffmpeg, kind string, seconds int) (stop func()) {
	t.Helper()
	cmd := exec.Command(ffmpeg, ffmpegArgs(kind, seconds)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start ffmpeg %s capture: %v", kind, err)
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

// TestCameraDetectionRequiresHardware captures from the real camera for a
// few seconds and verifies that monitor.CameraInUse reports it. Skipped when
// ffmpeg or a camera is not available.
func TestCameraDetectionRequiresHardware(t *testing.T) {
	if monitor.CameraInUse() {
		t.Skip("camera already in use by another application")
	}
	ffmpeg := ffmpegPath(t)
	probeDevice(t, ffmpeg, "camera")

	stop := stream(t, ffmpeg, "camera", 10)
	defer stop()

	waitFor(t, "camera detected as in use", 8*time.Second, monitor.CameraInUse)

	// Let the capture end and verify the state flips back.
	stop()
	waitFor(t, "camera detected as free after capture ended", 8*time.Second,
		func() bool { return !monitor.CameraInUse() })
}

// TestMicrophoneDetectionRequiresHardware captures from the real microphone
// for a few seconds and verifies that monitor.MicrophoneInUse reports it.
// Skipped when ffmpeg or a microphone is not available.
func TestMicrophoneDetectionRequiresHardware(t *testing.T) {
	if monitor.MicrophoneInUse() {
		t.Skip("microphone already in use by another application")
	}
	ffmpeg := ffmpegPath(t)
	probeDevice(t, ffmpeg, "mic")

	stop := stream(t, ffmpeg, "mic", 10)
	defer stop()

	waitFor(t, "microphone detected as in use", 8*time.Second, monitor.MicrophoneInUse)

	stop()
	waitFor(t, "microphone detected as free after capture ended", 8*time.Second,
		func() bool { return !monitor.MicrophoneInUse() })
}
