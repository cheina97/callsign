# ESP32 firmware: development environment setup

This guide sets up **PlatformIO** (build, upload, serial monitor, debugging)
and **Wokwi** (local ESP32 + MAX7219 simulation) for the callsign firmware,
all inside VS Code.

## 1. Install the tools

### PlatformIO

```sh
# VS Code extension (recommended - gives you the sidebar, buttons, debugger)
code --install-extension platformio.platformio-ide

# CLI (used by the Makefile targets)
pip3 install --user platformio
# then add the user bin dir to your PATH, e.g. in ~/.zshrc:
#   export PATH="$PATH:$(python3 -m site --user-base)/bin"
```

Verify:

```sh
pio --version     # -> PlatformIO Core, version 6.x
```

### Wokwi

1. Install the VS Code extension:
   ```sh
   code --install-extension wokwi.wokwi-vscode
   ```
2. Get a free license at <https://wokwi.com> (sign in, then
   `Wokwi: Request a new License` from the VS Code command palette).

## 2. Build the firmware

```sh
make firmware-build        # or: cd firmware && pio run
```

The first run downloads the ESP32 platform and toolchain (~1.5-2 GB into
`~/.platformio`) - subsequent builds take seconds.

**Check it works:** the build ends with something like

```
RAM:   [=         ]  14.5% (used 47664 bytes from 327680 bytes)
Flash: [========  ]  77.8% (used 1019456 bytes from 1310720 bytes)
========================= [SUCCESS] =========================
```

and produces `firmware/.pio/build/esp32dev/firmware.bin` (verified on this
repo with PlatformIO 6.2).

## 3. Simulate with Wokwi (no hardware needed)

The repo already contains the simulation setup:

- [wokwi.toml](wokwi.toml) - points Wokwi at the PlatformIO build output
- [diagram.json](diagram.json) - wires the ESP32 (pins 23/18/5 = DIN/CLK/CS)
  to a 4-module MAX7219 LED matrix

Steps:

1. Build first (`make firmware-build`) - Wokwi runs the compiled binary.
2. Open the command palette: `Wokwi: Start Simulator`.
3. The simulated ESP32 boots, connects to the simulated Wi-Fi
   (`Wokwi-GUEST`, no password - the sketch's hardcoded credentials are
   ignored in simulation... see note below), and starts the HTTP server.

> **Note on Wi-Fi credentials:** Wokwi provides a virtual network called
> `Wokwi-GUEST`. For the simulation to connect, set in the repo-root `.env`:
> ```sh
> WIFI_SSID=Wokwi-GUEST
> WIFI_PASSWORD=
> ```
> and rebuild (`make firmware-build`). Restore your real credentials
> before flashing to hardware.

**Check it works:**

- The virtual LED matrix shows nothing (initial state is `off`).
- From the Wokwi serial monitor you see `HTTP server started on port 80`.
- Wokwi forwards the simulated network: open the URL shown in the simulator
  (or use the `Wokwi: Open in browser` action) and:

```sh
# state -> call, matrix shows CALL
curl -X POST http://localhost:8180/status -d '{"state":"call"}'
curl http://localhost:8180/status
# -> {"state":"call","lastUpdateMs":12345}
```

(Port 8180 is Wokwi's default forwarding for the simulated port 80; check the
simulator output for the exact address.)

## 4. Flash a real ESP32

1. Set your real Wi-Fi credentials in the repo-root `.env`
   (`WIFI_SSID` / `WIFI_PASSWORD` - see `.env.example`). The build
   generates `firmware/callsign/build_config.h` from it automatically.
2. Connect the ESP32 via USB.
3. ```sh
   make firmware-upload
   make firmware-monitor     # serial output: assigned IP address
   ```

**Check it works:**

```sh
curl -X POST http://<esp32-ip>/status -d '{"state":"call"}'   # matrix: CALL
curl -X POST http://<esp32-ip>/status -d '{"state":"free"}'   # matrix: FREE
curl http://<esp32-ip>/status                                 # JSON status
```

Then start the client (`make run-client`) and watch the display follow your
camera/microphone state.

## 5. Tests & static analysis

```sh
make firmware-test     # host-side unit tests (display logic), no ESP32 needed
make firmware-check    # cppcheck static analysis
```

The unit tests compile the pure display logic
([callsign/display_logic.h](callsign/display_logic.h)) for your Mac with
PlatformIO's `native` platform and run it with the Unity test framework -
no hardware involved. Test sources live in [test/](test/).

## 6. Debugging

- **Simulation**: the Wokwi extension supports breakpoints/stepping in the
  simulated firmware (GDB-based, works with the ELF built by PlatformIO).
- **Real hardware**: classic ESP32 has no USB-JTAG, so on-chip debugging
  needs an ESP-Prog adapter wired to the JTAG pins. In practice, `Serial.printf`
  + `make firmware-monitor` covers most needs for this sketch.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `pio: command not found` | Restart the terminal after install, or use the PlatformIO sidebar in VS Code |
| First build fails downloading the platform | Check network/proxy; retry - downloads resume |
| Wokwi can't find the firmware | Run `make firmware-build` first (paths in `wokwi.toml` point at the build output) |
| Simulation boots but never connects to Wi-Fi | Use `Wokwi-GUEST` / empty password (see note above) |
| `curl: connection refused` in simulation | Check the forwarded port in the simulator output (default 8180) |
| Upload fails: `Failed to connect` | Hold the ESP32's BOOT button while `pio run -t upload` starts |
