// Host-side unit tests for the display logic (no ESP32 needed).
// Run with: make firmware-test   (or: cd firmware && pio test -e native)
#include <unity.h>

#include "display_logic.h"

void setUp(void) {}
void tearDown(void) {}

void test_glyph_for_known_letters(void) {
  TEST_ASSERT_NOT_NULL(glyphFor('A'));
  TEST_ASSERT_NOT_NULL(glyphFor('C'));
  TEST_ASSERT_NOT_NULL(glyphFor('E'));
  TEST_ASSERT_NOT_NULL(glyphFor('F'));
  TEST_ASSERT_NOT_NULL(glyphFor('L'));
  TEST_ASSERT_NOT_NULL(glyphFor('R'));
}

void test_glyph_for_unknown_char_returns_null(void) {
  TEST_ASSERT_NULL(glyphFor('Z'));
  TEST_ASSERT_NULL(glyphFor(' '));
  TEST_ASSERT_NULL(glyphFor('1'));
}

void test_render_call_is_centered(void) {
  uint8_t cols[DISPLAY_COLS];
  renderText("CALL", cols);

  // "CALL" is 4 glyphs * 7 px pitch - 1 = 27 px wide, margin 5 px split
  // 2 left / 3 right -> x = 5/2 = 2. Columns 0-1 empty, glyph at x = 2.
  for (int i = 0; i < 2; i++) {
    TEST_ASSERT_EQUAL_UINT8(0, cols[i]);
  }
  TEST_ASSERT_EQUAL_UINT8(FONT[1][0], cols[2]);  // 'C' first column
}

void test_render_call_matches_font(void) {
  uint8_t cols[DISPLAY_COLS];
  renderText("CALL", cols);

  // 'C' at x=2, 'A' at x=9, 'L' at x=16, 'L' at x=23 (7 px pitch).
  const uint8_t* glyphs[] = {FONT[1], FONT[0], FONT[4], FONT[4]};
  for (int g = 0; g < 4; g++) {
    for (int c = 0; c < GLYPH_COLS; c++) {
      TEST_ASSERT_EQUAL_UINT8(glyphs[g][c], cols[2 + g * 7 + c]);
    }
    // 1 px gap between glyphs (except after the last one).
    if (g < 3) {
      TEST_ASSERT_EQUAL_UINT8(0, cols[2 + g * 7 + GLYPH_COLS]);
    }
  }
}

void test_render_unknown_chars_are_skipped(void) {
  uint8_t cols[DISPLAY_COLS];
  renderText("A Z", cols);  // 'Z' and ' ' have no glyph

  // Width from the full string (3 chars -> 20 px, margin 12 -> x = 6),
  // but only 'A' is drawn, at x = 6.
  TEST_ASSERT_EQUAL_UINT8(FONT[0][0], cols[6]);
  TEST_ASSERT_EQUAL_UINT8(FONT[0][GLYPH_COLS - 1], cols[6 + GLYPH_COLS - 1]);
}

void test_render_mirrored(void) {
  uint8_t cols[DISPLAY_COLS];
  renderText("CALL", cols, true);

  // Same layout as test_render_call_matches_font but mirrored to 31-x.
  const uint8_t* glyphs[] = {FONT[1], FONT[0], FONT[4], FONT[4]};
  for (int g = 0; g < 4; g++) {
    for (int c = 0; c < GLYPH_COLS; c++) {
      TEST_ASSERT_EQUAL_UINT8(glyphs[g][c], cols[31 - (2 + g * 7 + c)]);
    }
  }
}

void test_render_clears_buffer_first(void) {
  uint8_t cols[DISPLAY_COLS];
  memset(cols, 0xFF, sizeof(cols));  // dirty buffer
  renderText("", cols);
  for (int i = 0; i < DISPLAY_COLS; i++) {
    TEST_ASSERT_EQUAL_UINT8(0, cols[i]);
  }
}

int main(int argc, char** argv) {
  (void)argc;
  (void)argv;
  UNITY_BEGIN();
  RUN_TEST(test_glyph_for_known_letters);
  RUN_TEST(test_glyph_for_unknown_char_returns_null);
  RUN_TEST(test_render_call_is_centered);
  RUN_TEST(test_render_call_matches_font);
  RUN_TEST(test_render_unknown_chars_are_skipped);
  RUN_TEST(test_render_mirrored);
  RUN_TEST(test_render_clears_buffer_first);
  return UNITY_END();
}
