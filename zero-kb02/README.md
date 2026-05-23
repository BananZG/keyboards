# zero-kb02 Firmware

Custom firmware for the **zero-kb02** — a compact 4×3 macro pad built around the Waveshare RP2040-Zero microcontroller. Written in [TinyGo](https://tinygo.org/) using the [sago35/tinygo-keyboard](https://github.com/sago35/tinygo-keyboard) framework.

---

## Hardware

| Component | Detail |
|-----------|--------|
| MCU | Waveshare RP2040-Zero |
| Key matrix | 4 columns × 3 rows (12 keys total) |
| LEDs | 12 × WS2812B RGB (GPIO 1) |
| Display | 128×64 SSD1306 OLED via I²C (GPIO 12/13) |
| Rotary encoder | Alps EC11 with push button (GPIO 3/4 + GPIO 2) |
| Joystick | Analog stick (ADC GPIO 28/29) + push button (GPIO 0) |

---

## Physical Key Layout

```
┌──────┬──────┬──────┬──────┐
│  K1  │  K2  │  K3  │  K4  │  ← Row 0
├──────┼──────┼──────┼──────┤
│  K5  │  K6  │  K7  │  K8  │  ← Row 1
├──────┼──────┼──────┼──────┤
│  FN  │  ⌫   │ 🖱 L  │ 🖱 R  │  ← Row 2 (fixed)
└──────┴──────┴──────┴──────┘
```

**Row 2 is always fixed** regardless of the active page:

| Key | Function |
|-----|----------|
| FN | Hold to enter page-selection mode |
| ⌫ | Backspace / Delete |
| 🖱 L | Mouse left click |
| 🖱 R | Mouse right click |

---

## Page System

Because the device only has 8 typeable keys (Rows 0–1), the firmware uses a **page system** to give access to the full alphabet, numbers, symbols, and navigation keys. Each page assigns a different set of characters to the 8 keys.

There are **11 pages** in total:

| # | Name | Row 0 (K1–K4) | Row 1 (K5–K8) |
|---|------|---------------|---------------|
| 0 | a-h  | a b c d | e f g h |
| 1 | i-p  | i j k l | m n o p |
| 2 | q-x  | q r s t | u v w x |
| 3 | y-z  | y z ␣ ↵ | , . / - |
| 4 | A-H  | A B C D | E F G H |
| 5 | I-P  | I J K L | M N O P |
| 6 | Q-X  | Q R S T | U V W X |
| 7 | Y-Z  | Y Z ␣ ↵ | : ; = ^ |
| 8 | 1-8  | 1 2 3 4 | 5 6 7 8 |
| 9 | 9-0  | 9 0 - . | ( ) = , |
| 10 | NAV | Home ↑ End PgUp | ← ↓ → PgDn |

---

## FN Key — Page Selection

**Hold FN** (bottom-left key) to enter page-selection mode.

While FN is held, the 11 keys act as **page shortcuts** — the layout mirrors the page table above:

```
┌────────┬────────┬────────┬────────┐
│  a-h   │  i-p   │  q-x   │  y-z   │  ← Row 0: lowercase pages
├────────┼────────┼────────┼────────┤
│  A-H   │  I-P   │  Q-X   │  Y-Z   │  ← Row 1: uppercase pages
├────────┼────────┼────────┼────────┤
│ (hold) │  1-8   │  9-0   │  NAV   │  ← Row 2: number / nav pages
└────────┴────────┴────────┴────────┘
```

**Behaviour:**
- All regular key output is suppressed while FN is held — no accidental characters.
- The OLED shows **"FN / select:"** while waiting.
- When a page key is pressed, the OLED **previews the selected page name** for ~1.5 seconds, then returns to the FN overlay (so you can keep selecting).
- **Pressing the same page repeatedly resets the preview timer** — handy for confirming your selection.
- Releasing FN commits the page and shows the full page info screen.

---

## Rotary Encoder

The rotary encoder has **three modes**, cycled by pressing the encoder button:

| Layer | Knob CCW | Knob CW | OLED |
|-------|----------|---------|------|
| 0 — Volume | Volume Down | Volume Up | `VOL` |
| 1 — Cursor | ← Arrow | → Arrow | `CURSOR` |
| 2 — Brightness | Brightness Down | Brightness Up | `BRIGHT` |

The current layer name is shown on the OLED for ~1.6 seconds each time you switch.

---

## Joystick

| Action | Function |
|--------|----------|
| Tilt in any direction | Moves the mouse cursor (analog, proportional) |
| Press (click) | Launches the built-in mini-game (koebiten) |

> The game takes over the display and rotary encoder inputs. Power-cycle or reset to return to normal keyboard mode.

---

## RGB LEDs

12 WS2812B LEDs run a continuous **rainbow animation** in idle state, with each LED offset slightly in hue to create a flowing effect.

When a key is pressed, the corresponding LED **flashes bright white** and fades back to the rainbow over ~200 ms.

---

## OLED Display

The 128×64 OLED has five display states:

| State | Trigger | Content | Duration |
|-------|---------|---------|----------|
| Page info | Page change / FN release | Page name + key hint (`abcd efgh`) | 3.2 s, then screensaver |
| Layer info | Rotary layer change | Layer name (VOL / CURSOR / BRIGHT) | 1.6 s, then page info |
| FN overlay | FN held | `FN` + `select:` | Until key pressed or FN released |
| FN preview | FN held + page selected | Selected page name + key hint | 1.5 s, then back to FN overlay |
| Screensaver | Idle after page info | Bouncing pixel | Until next event |

---

## Customisation

All key assignments live in [`firmware/pages.go`](firmware/pages.go) — no other files need touching for typical customisation.

### Changing keys on an existing page

Edit the `keyPages` array. Each page is a `[2][4]Keycode` (2 rows × 4 columns):

```go
// PageLowerA — change 'd' to Tab
{
    {jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyTab}, // ← Row 0
    {jp.KeyE, jp.KeyF, jp.KeyG, jp.KeyH},   //   Row 1
},
```

### Uppercase / shifted characters

Use `lsft(jp.KeyX)` instead of a bare keycode. This encodes the key through the library's proper Shift-modifier path:

```go
{lsft(jp.KeyA), lsft(jp.KeyB), ...}  // produces A B ...
```

### Adding a new page

1. Add a constant before `TotalPages` in the `const` block.
2. Add a `[2][4]Keycode` entry to `keyPages` at the matching index.
3. Add a display name to `pageNames`.
4. Wire it to an FN shortcut in `fnPageKeys`.

### Available keycodes

All standard HID keycodes are available via the `jp` (Japanese layout) or base keycodes packages. Common ones:

```go
jp.KeyA … jp.KeyZ          // letters
jp.Key1 … jp.Key0          // digits
jp.KeySpace, jp.KeyEnter, jp.KeyBackspace
jp.KeyLeft, jp.KeyRight, jp.KeyUp, jp.KeyDown
jp.KeyHome, jp.KeyEnd, jp.KeyPageUp, jp.KeyPageDown
jp.KeyComma, jp.KeyPeriod, jp.KeySlash, jp.KeyMinus
jp.MouseLeft, jp.MouseRight
jp.KeyMediaVolumeInc, jp.KeyMediaVolumeDec
jp.KeyMediaBrightnessUp, jp.KeyMediaBrightnessDown
```

---

## Building & Flashing

From the repository root:

```bash
# Build only
make build KEY=zero-kb02

# Flash (put device in bootloader mode first — hold BOOT and tap RESET)
make flash KEY=zero-kb02

# Build and flash in one step
make build-flash KEY=zero-kb02
```

The compiled `.uf2` is written to `out/zero-kb02.uf2`.

**Build size** (approximate): ~209 KB flash, ~47 KB RAM.
