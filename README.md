# callsign

"Am I on a call?" status display: an ESP32 drives a MAX7219 4-in-1 (32x8)
LED matrix showing **CALL** or **FREE**, updated by a Go client running on
your Mac that watches camera/microphone usage.

```
┌─────────────┐   POST /status    ┌────────┐  SPI   ┌──────────────────┐
│ Mac client  │ ────────────────► │ ESP32  │ ──────► │ MAX7219 32x8 LED │
│ (Go, events)│  {"state":"call"} │  Wi-Fi │          │ CALL / FREE      │
└─────────────┘                   └────────┘          └──────────────────┘
```

## How it works

- The client registers **OS-level event listeners** and reacts the instant the
  camera or microphone is opened/closed by any app — no polling.
- It only sends an update when the state **changes** (`call` ↔ `free`).
- It only sends when the Mac is on the **expected Wi-Fi network** (the one the
  ESP32 is on) — from any other network the request would be useless
  (skippable with `-no-wifi-check`).
- A **keep-alive** re-sends the current state every `KEEPALIVE_INTERVAL`
  (default 4 minutes), so the ESP32's auto-off only kicks in when the client
  is actually gone (stopped, Mac asleep, off the network) — not when the
  state is simply unchanged.
- If the ESP32 receives **no update for `FIRMWARE_TIMEOUT_S` seconds**
  (default 5 minutes), it turns the display off. Keep `KEEPALIVE_INTERVAL`
  shorter than `FIRMWARE_TIMEOUT_S`.

## Firmware (`firmware/callsign/callsign.ino`)

1. Install the **MD_MAX72XX** library by majicDesigns (Arduino Library Manager).
2. Open `firmware/callsign/callsign.ino` in the Arduino IDE.
3. Set `WIFI_SSID` / `WIFI_PASSWORD` in the repo-root `.env` (see `.env.example`).
4. Wire the matrix (default pins, changeable at the top of the file):

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
> both CALL and FREE show in the same color. The firmware uses the bold 6x8
> font from `display_logic.h` for maximum readability.

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
    ├── monitor/          # camera/microphone detection (CoreMediaIO/CoreAudio)
    └── network/          # Wi-Fi SSID detection
```

```sh
cd client
go build -o callsign-client ./cmd/callsign-client
./callsign-client -esp32 http://<esp32-ip> -ssid "YourHomeWifi"
```

Flags:

| Flag             | Default               | Meaning |
|------------------|-----------------------|---------|
| `-esp32`         | `http://192.168.1.50` | ESP32 base URL |
| `-ssid`          | `YOUR_WIFI_SSID`      | Wi-Fi network where the ESP32 lives |
| `-timeout`       | `5s`                  | HTTP timeout |
| `-no-wifi-check` | `false`               | Skip the Wi-Fi SSID check, always report |
| `-keepalive`     | `4m`                  | Re-send the current state at this interval; `0` disables |

Detection details (macOS):

- **Camera / Microphone**: queries the native CoreMediaIO / CoreAudio device
  state via [go-media-devices-state](https://github.com/cheina97/go-media-devices-state)
  (a fork of [antonfisher/go-media-devices-state](https://github.com/antonfisher/go-media-devices-state),
  wired in with a `replace` directive in `client/go.mod`). This reports
  whether any process currently holds a device open and works for any app
  (native, browser, Electron). Requires cgo.
- **Wi-Fi**: reads the current SSID via `ipconfig getsummary en0` (near-instant).
  macOS hides the SSID by default — if the client logs a warning that it
  cannot read the SSID, run `sudo ipconfig sethidewifiinfo 0` once to allow it.

### Testing without hardware

Run the fake server, which emulates the ESP32 REST API (including the
auto-off timeout) and logs every state change:

```sh
go run ./cmd/fake-server -addr :8080          # -timeout 5m by default
./callsign-client -esp32 http://localhost:8080 -ssid "YourHomeWifi"
```

### Testing camera/microphone detection

To verify detection alone (no server, no display), run `monitor-debug`: it
registers OS-level event listeners and prints a line the instant the camera
or microphone state changes:

```sh
go run ./cmd/monitor-debug
```

Open/close an app that uses the camera or microphone (e.g. Photo Booth,
Zoom) and you should see the transitions logged immediately.

### End-to-end tests

The `client/e2e` package runs the fake-server binary and drives it through
the display client, verifying the REST API contract, state transitions,
error handling and the auto-off watchdog:

```sh
make test-e2e     # or: cd client && go test -v ./e2e/
```

The same package also contains **hardware tests** for camera/microphone
detection: they capture a few seconds from the real device with
[ffmpeg](https://ffmpeg.org) (`brew install ffmpeg`) and assert that
`monitor.CameraInUse()` / `monitor.MicrophoneInUse()` react. On machines
without ffmpeg or a usable camera/microphone (CI runners, VMs) they **skip
automatically**, so `make test-e2e` is safe to run anywhere.

## Configuration (.env)

Both binaries read a `.env` file from the working directory (see
[.env.example](.env.example)). Priority: **CLI flag > env var / .env > default**.

```sh
cp .env.example .env   # then edit
```

| Variable    | Used by       | Meaning |
|-------------|---------------|---------|
| `ESP32_URL` | callsign-client | ESP32 (or fake server) base URL |
| `WIFI_SSID` | callsign-client + firmware | Wi-Fi network the ESP32 joins / client checks |
| `SKIP_WIFI_CHECK` | callsign-client | `true` skips the Wi-Fi SSID check |
| `KEEPALIVE_INTERVAL` | callsign-client | How often to re-send the current state (e.g. `4m`); `0` disables |
| `ADDR`      | fake-server    | Listen address |
| `WIFI_PASSWORD` | firmware   | Wi-Fi password baked into the firmware |
| `FIRMWARE_MIRROR_DISPLAY` | firmware | `true` mirrors the text (differently-wired matrices) |
| `FIRMWARE_TIMEOUT_S` | firmware | Seconds without updates before the display turns off |

The firmware variables are baked in at build time:
[firmware/gen_wifi_config.py](firmware/gen_wifi_config.py) generates
`firmware/callsign/build_config.h` from `.env` on every `make firmware-build`.

## Build & run (Makefile)

```sh
make help            # list all targets
make build           # build client + fake-server into bin/
make run-server      # run the fake server locally
make run-client      # run the client (uses .env)
make test vet fmt    # quality checks
```

Firmware targets (see [firmware/SETUP.md](firmware/SETUP.md) for the full guide):

```sh
make firmware-build    # compile the firmware (also needed before Wokwi)
make firmware-test     # host-side unit tests for the display logic
make firmware-check    # cppcheck static analysis
make firmware-size     # firmware size vs board limits
make firmware-upload   # flash a connected ESP32
make firmware-monitor  # serial monitor
make firmware-clean    # remove build artifacts
```

## Run the client in background (launchd)

The client is macOS-only, so the native way to keep it always running is a
**launchd LaunchAgent**: it starts at login and is restarted automatically by
macOS if it crashes.

```sh
cp .env.example .env   # configure ESP32_URL and WIFI_SSID first
make agent-install     # build, install and start the agent
make agent-status      # check it is running
tail -f /tmp/callsign.log
make agent-uninstall   # stop and remove
```

The plist template lives in [client/launchd/com.callsign.client.plist](client/launchd/com.callsign.client.plist);
`make agent-install` fills in the project path and loads it into
`~/Library/LaunchAgents`. `KeepAlive=true` makes launchd restart the process
whenever it exits unexpectedly (throttled to one restart per 10s).
