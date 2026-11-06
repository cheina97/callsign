// Package e2e contains end-to-end tests for the callsign system: they run
// the fake-server binary (which emulates the ESP32 REST API) and drive it
// through the display client, verifying the full request/response cycle,
// state transitions and the auto-off watchdog behavior.
package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"callsign-client/internal/display"
)

// statusResponse mirrors the JSON returned by GET/POST /status.
type statusResponse struct {
	State        string `json:"state"`
	LastUpdateMs int64  `json:"lastUpdateMs"`
}

// server is a running fake-server process.
type server struct {
	cmd     *exec.Cmd
	baseURL string
}

// startServer builds the fake-server binary (once per test run) and starts it
// on a free port with the given extra args (e.g. "-timeout", "200ms").
func startServer(t *testing.T, args ...string) *server {
	t.Helper()

	port := freePort(t)
	fullArgs := append([]string{"-addr", fmt.Sprintf("127.0.0.1:%d", port)}, args...)

	cmd := exec.Command(fakeServerBin(t), fullArgs...)
	// Surface server output in test logs on failure.
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start fake-server: %v", err)
	}

	s := &server{cmd: cmd, baseURL: fmt.Sprintf("http://127.0.0.1:%d", port)}
	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	waitForServer(t, s.baseURL)
	return s
}

// fakeServerBin builds the fake-server binary once and returns its path.
func fakeServerBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(os.TempDir(), "callsign-fake-server-e2e")
	build := exec.Command("go", "build", "-o", bin, "./cmd/fake-server")
	build.Dir = clientDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build fake-server: %v\n%s", err, out)
	}
	return bin
}

// clientDir returns the path of the client module (parent of this package).
func clientDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// freePort asks the OS for an available TCP port.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// waitForServer polls GET /status until the server responds or the deadline passes.
func waitForServer(t *testing.T, baseURL string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/status")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fake-server at %s did not become ready", baseURL)
}

// getStatus performs a raw GET /status and decodes the response.
func getStatus(t *testing.T, baseURL string) statusResponse {
	t.Helper()
	resp, err := http.Get(baseURL + "/status")
	if err != nil {
		t.Fatalf("GET /status: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /status: unexpected status %s", resp.Status)
	}
	var st statusResponse
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		t.Fatalf("decode /status response: %v", err)
	}
	return st
}

func TestInitialStateIsOff(t *testing.T) {
	s := startServer(t)
	st := getStatus(t, s.baseURL)
	if st.State != "off" {
		t.Errorf("initial state = %q, want %q", st.State, "off")
	}
}

func TestClientStateTransitions(t *testing.T) {
	s := startServer(t)
	client := display.NewClient(s.baseURL, 2*time.Second)

	for _, state := range []display.State{display.StateCall, display.StateFree, display.StateCall, display.StateOff} {
		if err := client.SetState(state); err != nil {
			t.Fatalf("SetState(%q): %v", state, err)
		}
		st := getStatus(t, s.baseURL)
		if st.State != string(state) {
			t.Errorf("after SetState(%q): state = %q", state, st.State)
		}
	}
}

func TestLastUpdateMsAdvances(t *testing.T) {
	s := startServer(t)
	client := display.NewClient(s.baseURL, 2*time.Second)

	if err := client.SetState(display.StateCall); err != nil {
		t.Fatal(err)
	}
	first := getStatus(t, s.baseURL).LastUpdateMs

	time.Sleep(50 * time.Millisecond)

	if err := client.SetState(display.StateFree); err != nil {
		t.Fatal(err)
	}
	second := getStatus(t, s.baseURL).LastUpdateMs

	if second <= first {
		t.Errorf("lastUpdateMs did not advance: first=%d second=%d", first, second)
	}
}

func TestRejectsInvalidState(t *testing.T) {
	s := startServer(t)
	client := display.NewClient(s.baseURL, 2*time.Second)

	if err := client.SetState(display.State("bogus")); err == nil {
		t.Error("SetState with invalid state: expected error, got nil")
	}

	// The server state must be unchanged.
	if st := getStatus(t, s.baseURL); st.State != "off" {
		t.Errorf("state after rejected POST = %q, want %q", st.State, "off")
	}
}

func TestRejectsMalformedJSON(t *testing.T) {
	s := startServer(t)

	resp, err := http.Post(s.baseURL+"/status", "application/json",
		http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("POST with empty body: status = %s, want 400", resp.Status)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s := startServer(t)

	req, _ := http.NewRequest(http.MethodDelete, s.baseURL+"/status", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /status: status = %s, want 405", resp.Status)
	}
}

func TestAutoOffAfterTimeout(t *testing.T) {
	// Start the server with a very short watchdog timeout.
	s := startServer(t, "-timeout", "300ms")
	client := display.NewClient(s.baseURL, 2*time.Second)

	if err := client.SetState(display.StateCall); err != nil {
		t.Fatal(err)
	}
	if st := getStatus(t, s.baseURL); st.State != "call" {
		t.Fatalf("state = %q, want %q", st.State, "call")
	}

	// The watchdog ticks every 5s in the fake server, so wait generously.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st := getStatus(t, s.baseURL); st.State == "off" {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Error("server did not auto-off after timeout without updates")
}

func TestClientAgainstUnreachableServer(t *testing.T) {
	// No server listening on this port.
	port := freePort(t)
	client := display.NewClient(fmt.Sprintf("http://127.0.0.1:%d", port), 500*time.Millisecond)

	if err := client.SetState(display.StateCall); err == nil {
		t.Error("SetState against unreachable server: expected error, got nil")
	}
}
