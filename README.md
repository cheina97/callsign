# callsign

"Am I on a call?" status display: an ESP32 drives a MAX7219 4-in-1 (32x8) LED
matrix showing **CALL** or **FREE**, updated by a Go client running on your Mac
that watches camera/microphone usage.

```
┌─────────────┐   POST /status    ┌────────┐   SPI   ┌──────────────────┐
│ Mac client  │ ────────────────► │ ESP32  │ ──────► │ MAX7219 32x8 LED │
│ (Go, 1/min) │  {"state":"call"} │  Wi-Fi │         │  CALL / FREE     │
└─────────────┘                   └────────┘         └──────────────────┘
```

## How it works

- The client checks every minute whether the camera or microphone is in use.
- It only sends an update when the state **changes** (`call` ↔ `free`).
- It only sends when the Mac is on the **expected Wi-Fi network** (the one the
  ESP32 is on) — from any other network the request would be useless.
- If the ESP32 receives **no update for 5 minutes**, it turns the display off
  (e.g. Mac asleep, away from the network, client stopped).

## Firmware (`firmware/callsign/callsign.ino`)

1. Install the **MD_MAX72XX** library by majicDesigns (Arduino Library Manager).
2. Open `firmware/callsign/callsign.ino` in the Arduino IDE.
3. Set `WIFI_SSID` / `WIFI_PASSWORD`.
4. Wire the module (default pins, changeable at the top of the file):

   | MAX7219 | ESP32 |
   |---------|-------|
   | VCC     | 5V (VIN) |
   | GND     | GND |
   | DIN     | GPIO 23 |
   | CS      | GPIO 5 |
   | CLK     | GPIO 18 |

5. Flash, then open the Serial Monitor to see the assigned IP (or give the
   ESP32 a DHCP reservation in your router).

### REST API

| Method | Path      | Body                              | Response |
|--------|-----------|-----------------------------------|----------|
| GET    | `/status` | —                                 | `{"state":"call\|free\|off","lastUpdateMs":123}` |
| POST   | `/status` | `{"state":"call"}` / `"free"` / `"off"` | same as GET |

Manual test:

```sh
curl -X POST http://<esp32-ip>/status -d '{"state":"call"}'
```

> **Note on color:** MAX7219 4-in-1 modules are single-color (usually red), so
> "CALL in red / FREE in green" can't be shown as two colors on this hardware.
> The firmware shows the text; if you later add a second (green) display or a
> bi-color matrix, `applyState()` is the single place to extend.

## Client (`client/`)

Layout:

```
client/
├── cmd/
│   ├── callsign-client/  # the monitor client
│   ├── fake-server/      # fake ESP32 API for testing without hardware
│   └── monitor-debug/    # prints camera/mic state changes (no server needed)
├── e2e/                  # end-to-end tests (fake server + display client)
└── internal/
    ├── display/          # ESP32 REST API client
    ├── monitor/          # camera/microphone detection (macOS unified log)
    └── network/          # Wi-Fi SSID detection
```

```sh
cd client
go build -o callsign-client ./cmd/callsign-client
./callsign-client -esp32 http://<esp32-ip> -ssid "YourHomeWifi"
```

Flags:

| Flag        | Default               | Meaning |
|-------------|-----------------------|---------|
| `-esp32`    | `http://192.168.1.50` | ESP32 base URL |
| `-ssid`     | `YOUR_WIFI_SSID`      | Wi-Fi network where the ESP32 lives |
| `-interval` | `1m`                  | Check interval |
| `-timeout`  | `5s`                  | HTTP timeout |

Detection details (macOS):

- **Camera**: scans the last 2 minutes of unified logs for
  `VDCAssistant_Power_State = On` (com.apple.cmio).
- **Microphone**: scans for CoreAudio `HALS_Device` entries with
  `isInput = 1` and `isRunning = 1`.
- **Wi-Fi**: reads the current SSID via `system_profiler SPAirPortDataType`
  (works on macOS 14+ where `airport -I` no longer returns the SSID).

### Testing without hardware

Run the fake server, which emulates the ESP32 REST API (including the
auto-off timeout) and logs every state change:

```sh
go run ./cmd/fake-server -addr :8080          # -timeout 5m by default
./callsign-client -esp32 http://localhost:8080 -ssid "YourHomeWifi"
```

### Testing camera/microphone detection

To verify detection alone (no server, no display), run `monitor-debug`: it
polls the camera/microphone state and prints a line every time it changes:

```sh
go run ./cmd/monitor-debug            # -interval 2s by default
```

Open/close an app that uses the camera or microphone (e.g. Photo Booth,
Zoom) and you should see the transitions logged.

### End-to-end tests

The `client/e2e` package runs the fake-server binary and drives it through
the display client, verifying the REST API contract, state transitions,
error handling and the auto-off watchdog:

```sh
make test-e2e     # or: cd client && go test -v ./e2e/
```

## Configuration (.env)

Both binaries read a `.env` file from the working directory (see
[.env.example](.env.example)). Priority: **CLI flag > env var / .env > default**.

```sh
cp .env.example .env   # then edit
```

| Variable    | Used by       | Meaning |
|-------------|---------------|---------|
| `ESP32_URL` | callsign-client | ESP32 (or fake server) base URL |
| `WIFI_SSID` | callsign-client | Wi-Fi network where the ESP32 lives |
| `ADDR`      | fake-server    | Listen address |

## Build & run (Makefile)

```sh
make help            # list all targets
make build           # build client + fake-server into bin/
make run-server      # run the fake server locally
make run-client      # run the client (uses .env)
make test vet fmt    # quality checks
```

## Run the client in background (launchd)

The client is macOS-only, so the native way to keep it always running is a
**launchd LaunchAgent**: it starts at login and is restarted automatically by
macOS if it crashes.

```sh
cp .env.example .env   # configure ESP32_URL and WIFI_SSID first
make install-agent     # build, install and start the agent
make agent-status      # check it is running
tail -f /tmp/callsign.log
make uninstall-agent   # stop and remove
```

The plist template lives in [launchd/com.callsign.client.plist](launchd/com.callsign.client.plist);
`make install-agent` fills in the project path and loads it into
`~/Library/LaunchAgents`. `KeepAlive=true` makes launchd restart the process
whenever it exits unexpectedly (throttled to one restart per 10s).
