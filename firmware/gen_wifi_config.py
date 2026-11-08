# Generates callsign/build_config.h from the repo-root .env so Wi-Fi
# credentials stay out of source control. Runs before every build
# (extra_scripts in platformio.ini). Real environment variables
# override .env values, matching the client tooling.
import os

Import("env")  # PlatformIO build environment (unused, but required hook)

FIRMWARE_DIR = env.subst("$PROJECT_DIR")
ENV_FILE = os.path.join(FIRMWARE_DIR, "..", ".env")
OUT_FILE = os.path.join(FIRMWARE_DIR, "callsign", "build_config.h")

DEFAULTS = {
    "WIFI_SSID": "YOUR_WIFI_SSID",
    "WIFI_PASSWORD": "YOUR_WIFI_PASSWORD",
    "FIRMWARE_MIRROR_DISPLAY": "false",
    "FIRMWARE_TIMEOUT_S": "300",  # 5 minutes
}


def load_env(path):
    values = {}
    if not os.path.exists(path):
        return values
    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue
            key, _, value = line.partition("=")
            values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def get(file_values, key):
    return os.environ.get(key, file_values.get(key, DEFAULTS[key]))


file_values = load_env(ENV_FILE)
ssid = get(file_values, "WIFI_SSID")
password = get(file_values, "WIFI_PASSWORD")
mirror = get(file_values, "FIRMWARE_MIRROR_DISPLAY").lower() in ("1", "true", "yes")
try:
    timeout_s = int(get(file_values, "FIRMWARE_TIMEOUT_S"))
except ValueError:
    print("gen_wifi_config: WARNING - invalid FIRMWARE_TIMEOUT_S, using 300")
    timeout_s = 300

content = (
    "// Build configuration. Generated from the repo-root .env by\n"
    "// firmware/gen_wifi_config.py at build time - do not edit by hand.\n"
    "#pragma once\n\n"
    f'#define WIFI_SSID "{ssid}"\n'
    f'#define WIFI_PASSWORD "{password}"\n'
    f"#define MIRROR_DISPLAY {1 if mirror else 0}\n"
    f"#define TIMEOUT_MS {timeout_s}UL * 1000UL\n"
)

# Only rewrite when the content changes, to avoid needless rebuilds.
current = None
if os.path.exists(OUT_FILE):
    with open(OUT_FILE) as f:
        current = f.read()

if current != content:
    with open(OUT_FILE, "w") as f:
        f.write(content)

if ssid == DEFAULTS["WIFI_SSID"]:
    print("gen_wifi_config: WARNING - using placeholder Wi-Fi credentials "
          "(set WIFI_SSID/WIFI_PASSWORD in .env)")
else:
    print(f"gen_wifi_config: build_config.h generated "
          f"(SSID: {ssid}, mirror: {'on' if mirror else 'off'}, "
          f"timeout: {timeout_s}s)")
