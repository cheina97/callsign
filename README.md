# callsign

"Am I on a call?" status display: an ESP32 drives a WS2812B (NeoPixel) 32x8
RGB LED matrix showing **CALL** (red) or **FREE** (green), updated by a Go
client running on your Mac that watches camera/microphone usage.

```
┌─────────────┐   POST /status    ┌────────┐  1-wire  ┌──────────────────┐
│ Mac client  │ ────────────────► │ ESP32  │ ───────► │ WS2812B 32x8 RGB │
│ (Go, events)│  {"state":"call"} │  Wi-Fi │          │ CALL red/FREE grn│
└─────────────┘                   └────────┘          └──────────────────┘
```

## How it works

- The client registers **OS-level event listeners** and reacts the instant the
  camera or microphone is opened/closed by any app — no polling.
- It only sends an update when the state **changes** (`call` ↔ `free`).
- It only sends when the Mac is on the **expected Wi-Fi network** (the one the
  ESP32 is on) — from any other network the request would be useless
  (skippable with `-no-wifi-check`).
- If the ESP32 receives **no update for `FIRMWARE_TIMEOUT_S` seconds**
  (default 5 minutes), it turns the display off (e.g. Mac asleep, away from
  the network, client stopped).

## Firmware (`firmware/callsign/callsign.ino`)

1. Install the **FastLED** library by Daniel Garcia (Arduino Library Manager).
2. Open `firmware/callsign/callsign.ino` in the Arduino IDE.
3. Set `WIFI_SSID` / `WIFI_PASSWORD`.
4. Wire the matrix (default pin, changeable at the top of the file):

   | WS2812B matrix | ESP32 |
   |----------------|-------|
   | VCC            | 5V (VIN) |
   | GND            | GND |
   | DIN            | GPIO 23 |

   > A 32x8 WS2812B matrix can draw several amps at full white. Power it from
   > a proper 5V supply (not the ESP32's regulator) for anything beyond dim
   > text, and add a ~470Ω resistor in series on DIN plus a 1000µF capacitor
   > across VCC/GND as recommended in the NeoPixel Überguide.

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

> **Colors:** the WS2812B matrix is full RGB. The firmware shows **CALL in
> red** and **FREE in green**; change the `CRGB::Red` / `CRGB::Green` values
> in `applyState()` to pick different colors.

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

Detection details (macOS):

- **Camera / Microphone**: queries the native CoreMediaIO / CoreAudio device
  state via [go-media-devices-state](https://github.com/cheina97/go-media-devices-state)
  (a fork of [antonfisher/go-media-devices-state](https://github.com/antonfisher/go-media-devices-state),
  wired in with a `replace` directive in `client/go.mod`). This reports
  whether any process currently holds a device open and works for any app
  (native, browser, Electron). Requires cgo.
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
