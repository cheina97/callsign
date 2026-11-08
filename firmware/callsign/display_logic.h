// Pure display logic for the 32x8 LED matrix: glyph table, text centering
// and column-buffer layout. No Arduino/hardware dependencies, so it can be
// unit-tested on the host (pio test -e native).
#pragma once

#include <stdint.h>
#include <string.h>

#define DISPLAY_COLS 32
#define DISPLAY_ROWS 8

#define GLYPH_COLS 6
#define GLYPH_GAP 1

// Bold 6x8 font, column-major, LSB = top pixel. Uses the full 8-row height
// of the matrix, with 2-pixel strokes on both axes. Only the glyphs we need.
const uint8_t FONT[][GLYPH_COLS] = {
  /* 'A' */ {0xFC, 0xFF, 0x33, 0x33, 0xFF, 0xFC},
  /* 'C' */ {0x7E, 0xFF, 0xC3, 0xC3, 0xE7, 0x66},
  /* 'E' */ {0xFF, 0xFF, 0xDB, 0xDB, 0xDB, 0xC3},
  /* 'F' */ {0xFF, 0xFF, 0x1B, 0x1B, 0x1B, 0x03},
  /* 'L' */ {0xFF, 0xFF, 0xC0, 0xC0, 0xC0, 0xC0},
  /* 'R' */ {0xFF, 0xFF, 0x1B, 0x1B, 0xFF, 0xF6},
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

// Renders text centered on the 32x8 display into a column buffer
// (cols[x] bit y = LED at column x, row y; row 0 = top).
// When mirror is true the buffer is flipped horizontally: some matrix
// variants are wired the other way around, making the text read backwards.
void renderText(const char* text, uint8_t cols[DISPLAY_COLS], bool mirror = false) {
  memset(cols, 0, DISPLAY_COLS);

  uint8_t len = strlen(text);
  uint8_t pitch = GLYPH_COLS + GLYPH_GAP;
  uint8_t textWidth = len * pitch - GLYPH_GAP;
  // Center the text; with an odd margin the extra pixel goes on the right
  // (e.g. 27 px text on 32 cols -> 2 px left, 3 px right).
  int8_t x = (DISPLAY_COLS - textWidth) / 2;

  for (uint8_t i = 0; i < len; i++) {
    const uint8_t* g = glyphFor(text[i]);
    if (g == nullptr) continue;
    for (uint8_t c = 0; c < GLYPH_COLS; c++) {
      int8_t pos = x + i * pitch + c;
      if (pos >= 0 && pos < DISPLAY_COLS) {
        cols[mirror ? DISPLAY_COLS - 1 - pos : pos] = g[c];
      }
    }
  }
}
