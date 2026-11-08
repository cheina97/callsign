/*
 * callsign - ESP32 + MAX7219 4-in-1 (32x8) status display
 *
 * REST API:
 *   GET  /status  -> {"state":"call|free|off","lastUpdateMs":<n>}
 *   POST /status  body: {"state":"call"} | {"state":"free"} | {"state":"off"}
 *
 * Behavior:
 *   - "call" shows CALL, "free" shows FREE, "off" blanks the display.
 *   - If no POST /status is received for TIMEOUT_MS (5 min), the display
 *     turns off automatically.
 *
 * Required library (Arduino Library Manager): "MD_MAX72XX" by majicDesigns
 * (installs the MD_Parola dependency automatically).
 */

#include <WiFi.h>
#include <WebServer.h>
#include <MD_MAX72xx.h>
#include <SPI.h>

// ---------------- Configuration ----------------
const char* WIFI_SSID     = "YOUR_WIFI_SSID";
const char* WIFI_PASSWORD = "YOUR_WIFI_PASSWORD";

#define HARDWARE_TYPE MD_MAX72XX::FC16_HW  // most common 4-in-1 module type
#define MAX_DEVICES   4
#define PIN_DIN       23
#define PIN_CLK       18
#define PIN_CS         5

const uint32_t TIMEOUT_MS = 5UL * 60UL * 1000UL;  // 5 minutes
// -------------------------------------------------

MD_MAX72XX mx(HARDWARE_TYPE, PIN_CS, MAX_DEVICES);
WebServer server(80);

enum State { STATE_OFF, STATE_CALL, STATE_FREE };
State currentState = STATE_OFF;
uint32_t lastUpdateMs = 0;

// 5x7 font, column-major, LSB = top pixel. Only the glyphs we need.
const uint8_t FONT[][5] = {
  /* 'A' */ {0x7E, 0x11, 0x11, 0x11, 0x7E},
  /* 'C' */ {0x3E, 0x41, 0x41, 0x41, 0x22},
  /* 'E' */ {0x7F, 0x49, 0x49, 0x49, 0x41},
  /* 'F' */ {0x7F, 0x09, 0x09, 0x09, 0x01},
  /* 'L' */ {0x7F, 0x40, 0x40, 0x40, 0x40},
  /* 'R' */ {0x7F, 0x09, 0x19, 0x29, 0x46},
};

const uint8_t* glyphFor(char c) {
  switch (c) {
    case 'A': return FONT[0];
    case 'C': return FONT[1];
    case 'E': return FONT[2];
    case 'F': return FONT[3];
    case 'L': return FONT[4];
    case 'R': return FONT[5];
  }
  return nullptr;
}

// Draws text centered on the 32x8 display.
// MD_MAX72XX point (0,0) is the top-left LED of the module chain.
void drawText(const char* text) {
  uint8_t cols[32];
  memset(cols, 0, sizeof(cols));

  uint8_t len = strlen(text);
  uint8_t textWidth = len * 6 - 1;  // 5 px glyph + 1 px spacing
  int8_t x = (32 - textWidth) / 2;

  for (uint8_t i = 0; i < len; i++) {
    const uint8_t* g = glyphFor(text[i]);
    if (g == nullptr) continue;
    for (uint8_t c = 0; c < 5; c++) {
      int8_t pos = x + i * 6 + c;
      if (pos >= 0 && pos < 32) cols[pos] = g[c];
    }
  }

  mx.control(MD_MAX72XX::UPDATE, MD_MAX72XX::OFF);
  mx.clear();
  for (uint8_t col = 0; col < 32; col++) {
    uint8_t device = col / 8;   // device 0 = leftmost module
    uint8_t devCol = col % 8;
    for (uint8_t row = 0; row < 8; row++) {
      if (cols[col] & (1 << row)) {
        mx.setPoint(row, device * 8 + devCol, true);
      }
    }
  }
  mx.control(MD_MAX72XX::UPDATE, MD_MAX72XX::ON);
}

void applyState(State s) {
  currentState = s;
  switch (s) {
    case STATE_CALL: drawText("CALL"); break;
    case STATE_FREE: drawText("FREE"); break;
    case STATE_OFF:  mx.clear();     break;
  }
}

const char* stateName(State s) {
  switch (s) {
    case STATE_CALL: return "call";
    case STATE_FREE: return "free";
    default:         return "off";
  }
}

void sendStatus() {
  char body[96];
  snprintf(body, sizeof(body),
           "{\"state\":\"%s\",\"lastUpdateMs\":%lu}",
           stateName(currentState), (unsigned long)lastUpdateMs);
  server.send(200, "application/json", body);
}

void handleGetStatus() {
  sendStatus();
}

void handlePostStatus() {
  String body = server.arg("plain");
  body.toLowerCase();

  if (body.indexOf("\"call\"") >= 0) {
    applyState(STATE_CALL);
  } else if (body.indexOf("\"free\"") >= 0) {
    applyState(STATE_FREE);
  } else if (body.indexOf("\"off\"") >= 0) {
    applyState(STATE_OFF);
  } else {
    server.send(400, "application/json",
                "{\"error\":\"expected {\\\"state\\\":\\\"call|free|off\\\"}\"}");
    return;
  }

  lastUpdateMs = millis();
  sendStatus();
}

void connectWiFi() {
  WiFi.mode(WIFI_STA);
  WiFi.begin(WIFI_SSID, WIFI_PASSWORD);
  Serial.printf("Connecting to %s", WIFI_SSID);
  while (WiFi.status() != WL_CONNECTED) {
    delay(500);
    Serial.print(".");
  }
  Serial.printf("\nConnected. IP: %s\n", WiFi.localIP().toString().c_str());
}

void setup() {
  Serial.begin(115200);
  mx.begin();
  mx.control(MD_MAX72XX::INTENSITY, 4);  // 0-15
  mx.clear();

  connectWiFi();

  server.on("/status", HTTP_GET, handleGetStatus);
  server.on("/status", HTTP_POST, handlePostStatus);
  server.begin();
  Serial.println("HTTP server started on port 80");
}

void loop() {
  if (WiFi.status() != WL_CONNECTED) {
    connectWiFi();
  }

  server.handleClient();

  // Turn the display off if no update arrived within the timeout.
  if (currentState != STATE_OFF &&
      (uint32_t)(millis() - lastUpdateMs) > TIMEOUT_MS) {
    Serial.println("No update for 5 minutes - turning display off");
    applyState(STATE_OFF);
  }
}
