// Package display is the client for the ESP32 status display REST API.
package display

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// State is a display state understood by the ESP32 firmware.
type State string

const (
	StateCall State = "call"
	StateFree State = "free"
	StateOff  State = "off"
)

// Client talks to the ESP32 REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a Client for the given ESP32 base URL (e.g. http://192.168.1.50).
func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: timeout},
	}
}

// SetState posts the given state to the display.
func (c *Client) SetState(state State) error {
	body, _ := json.Marshal(map[string]string{"state": string(state)})
	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/status", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}
