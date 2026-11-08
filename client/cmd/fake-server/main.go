// fake-server emulates the ESP32 status display REST API, so the client can
// be tested without hardware.
//
// It implements the same endpoints as the firmware:
//
//	GET  /status -> {"state":"call|free|off","lastUpdateMs":<n>}
//	POST /status <- {"state":"call|free|off"}
//
// State changes are logged to stdout, and -timeout replicates the firmware's
// auto-off behavior after a period without updates.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"callsign-client/internal/config"
)

var (
	addr        = flag.String("addr", ":8080", "Listen address (env: ADDR)")
	timeout     = flag.Duration("timeout", 5*time.Minute, "Turn off after this long without an update (like the firmware)")
	verboseJSON = flag.Bool("v", false, "Log full request bodies")
)

type server struct {
	mu           sync.Mutex
	state        string
	lastUpdateMs int64
	started      time.Time
}

func (s *server) millis() int64 {
	return time.Since(s.started).Milliseconds()
}

func (s *server) status(w http.ResponseWriter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"state":        s.state,
		"lastUpdateMs": s.lastUpdateMs,
	})
}

func (s *server) handleStatus(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.status(w)
	case http.MethodPost:
		var body struct {
			State string `json:"state"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		switch body.State {
		case "call", "free", "off":
		default:
			http.Error(w, `{"error":"expected {\"state\":\"call|free|off\"}"}`, http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		s.state = body.State
		s.lastUpdateMs = s.millis()
		s.mu.Unlock()

		log.Printf(">>> display: %-4s (t=%dms)", body.State, s.lastUpdateMs)
		s.status(w)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// watchdog replicates the firmware behavior: turn off after timeout without updates.
func (s *server) watchdog() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		idle := time.Duration(s.millis()-s.lastUpdateMs) * time.Millisecond
		if s.state != "off" && idle > *timeout {
			s.state = "off"
			log.Printf(">>> display: off  (no update for %s - timeout)", idle.Truncate(time.Second))
		}
		s.mu.Unlock()
	}
}

func main() {
	config.Load(".env")
	flag.Parse()
	config.ApplyEnv()
	s := &server{state: "off", started: time.Now()}

	mux := http.NewServeMux()
	mux.HandleFunc("/status", s.handleStatus)

	if *verboseJSON {
		log.Println("verbose mode on")
	}
	go s.watchdog()

	fmt.Printf("fake ESP32 display listening on %s (auto-off after %s without updates)\n", *addr, *timeout)
	fmt.Printf("point the client at it with: callsign-client -esp32 http://localhost%s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
