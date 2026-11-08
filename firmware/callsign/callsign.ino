/*
 * callsign - ESP32 + WS2812B (NeoPixel) 32x8 RGB status display
 *
 * REST API:
 *   GET  /status  -> {"state":"call|free|off","lastUpdateMs":<n>}
 *   POST /status  body: {"state":"call"} | {"state":"free"} | {"state":"off"}
 *
 * Behavior:
 *   - "call" shows CALL in red, "free" shows FREE in green, "off" blanks
 *     the display.
 *   - If no POST /status is received for TIMEOUT_MS, the display turns off
 *     automatically.
 *
 * Required library (Arduino Library Manager): "FastLED" by Daniel Garcia.
 */

#include <WiFi.h>
#include <WebServer.h>
#include <FastLED.h>

#include "display_logic.h" // glyph table + text rendering (unit-tested on host)
#include "build_config.h"  // WIFI_SSID / WIFI_PASSWORD / MIRROR_DISPLAY, generated from .env

// ---------------- Configuration ----------------
#define PIN_DATA 23
#define NUM_LEDS (DISPLAY_COLS * DISPLAY_ROWS) // 32x8 = 256
#define BRIGHTNESS 32                          // 0-255, WS2812B is blinding at full

// TIMEOUT_MS comes from build_config.h (FIRMWARE_TIMEOUT_S in .env)
// -------------------------------------------------

CRGB leds[NUM_LEDS];
WebServer server(80);

enum State
{
  STATE_OFF,
  STATE_CALL,
  STATE_FREE
};
State currentState = STATE_OFF;
uint32_t lastUpdateMs = 0;

// Maps a (col, row) pixel to the LED index. WS2812B matrices are wired in a
// serpentine pattern: even columns run top-to-bottom, odd columns bottom-to-top.
uint16_t xy(uint8_t col, uint8_t row)
{
  if (col % 2 == 0)
  {
    return col * DISPLAY_ROWS + row;
  }
  return col * DISPLAY_ROWS + (DISPLAY_ROWS - 1 - row);
}

// Draws text centered on the 32x8 display in the given color.
void drawText(const char *text, CRGB color)
{
  uint8_t cols[DISPLAY_COLS];
  renderText(text, cols, MIRROR_DISPLAY != 0);

  FastLED.clear();
  for (uint8_t col = 0; col < DISPLAY_COLS; col++)
  {
    for (uint8_t row = 0; row < DISPLAY_ROWS; row++)
    {
      if (cols[col] & (1 << row))
      {
        leds[xy(col, row)] = color;
      }
    }
  }
  FastLED.show();
}

void applyState(State s)
{
  currentState = s;
  switch (s)
  {
  case STATE_CALL:
    drawText("CALL", CRGB::Red);
    break;
  case STATE_FREE:
    drawText("FREE", CRGB::Green);
    break;
  case STATE_OFF:
    FastLED.clear();
    FastLED.show();
    break;
  }
}

const char *stateName(State s)
{
  switch (s)
  {
  case STATE_CALL:
    return "call";
  case STATE_FREE:
    return "free";
  default:
    return "off";
  }
}

void sendStatus()
{
  char body[96];
  snprintf(body, sizeof(body),
           "{\"state\":\"%s\",\"lastUpdateMs\":%lu}",
           stateName(currentState), (unsigned long)lastUpdateMs);
  server.send(200, "application/json", body);
}

void handleGetStatus()
{
  sendStatus();
}

void handlePostStatus()
{
  String body = server.arg("plain");
  body.toLowerCase();

  if (body.indexOf("\"call\"") >= 0)
  {
    applyState(STATE_CALL);
  }
  else if (body.indexOf("\"free\"") >= 0)
  {
    applyState(STATE_FREE);
  }
  else if (body.indexOf("\"off\"") >= 0)
  {
    applyState(STATE_OFF);
  }
  else
  {
    server.send(400, "application/json",
                "{\"error\":\"expected {\\\"state\\\":\\\"call|free|off\\\"}\"}");
    return;
  }

  lastUpdateMs = millis();
  sendStatus();
}

void connectWiFi()
{
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.printf("Connecting to %s", WIFI_SSID);
  while (WiFi.status() != WL_CONNECTED)
  {
    delay(500);
    Serial.print(".");
  }
  Serial.printf("\nConnected. IP: %s\n", WiFi.localIP().toString().c_str());
}

void setup()
{
  Serial.begin(115200);
  FastLED.addLeds<WS2812B, PIN_DATA, GRB>(leds, NUM_LEDS);
  FastLED.setBrightness(BRIGHTNESS);
  FastLED.clear();
  FastLED.show();

  connectWiFi();

  server.on("/status", HTTP_GET, handleGetStatus);
  server.on("/status", HTTP_POST, handlePostStatus);
  server.begin();
  Serial.println("HTTP server started on port 80");
}

void loop()
{
  if (WiFi.status() != WL_CONNECTED)
  {
    connectWiFi();
  }

  server.handleClient();

  // Turn the display off if no update arrived within the timeout.
  if (currentState != STATE_OFF &&
      (uint32_t)(millis() - lastUpdateMs) > TIMEOUT_MS)
  {
    Serial.println("No update within timeout - turning display off");
    applyState(STATE_OFF);
  }
}
