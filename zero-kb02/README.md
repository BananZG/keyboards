# zero-kb02 Firmware

Custom firmware for the **zero-kb02** — a compact 4×3 macro pad built around the Waveshare RP2040-Zero microcontroller. Written in [TinyGo](https://tinygo.org/) using the [sago35/tinygo-keyboard](https://github.com/sago35/tinygo-keyboard) framework with [Vial](https://get.vial.today/) live key-remapping support.

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
│ Sft  │  K6  │  K7  │  K8  │  ← Row 1  (K5 = Left Shift)
├──────┼──────┼──────┼──────┤
│  FN  │  ⌫   │ 🖱 L  │ 🖱 R  │  ← Row 2 (always fixed)
└──────┴──────┴──────┴──────┘
```

**Row 2 is always fixed** across every layer:

| Key | Function |
|-----|----------|
| FN | Hold to enter layer-selection mode |
| ⌫ | Backspace |
| 🖱 L | Mouse left click |
| 🖱 R | Mouse right click |

---

## Layer System

The firmware uses **6 combined Vial layers**. Each layer is a complete, self-contained key configuration — the rotary encoder mode, the letter/number keys, and the Shift key position all live in the same layer. Switching layers instantly changes everything at once.

| Layer | OLED | Row 0 (K1–K4) | Row 1 (Sft, K6–K8) | Rotary knob |
|-------|------|---------------|---------------------|-------------|
| 0 | `L-0  VOL` | a b c d | **Shift** e f g | Volume ▲▼ |
| 1 | `L-1  SCR` | h i j k | **Shift** l m n | Scroll ▲▼ |
| 2 | `L-2  BRT` | o p q r | **Shift** s t u | Brightness ▲▼ |
| 3 | `L-3` | v w x y | **Shift** z 0 1 | *(unbound)* |
| 4 | `L-4` | 2 3 4 5 | 6 7 8 9 | *(unbound)* |
| 5 | `L-5` | Home ↑ End PgUp | ← ↓ → PgDn | *(unbound)* |

The **Shift key** at K5 is a standard HID Left Shift — hold it and press any letter key on the same layer to type the uppercase version, just like a regular keyboard.

---

## FN Key — Layer Selection

**Hold FN** (bottom-left, Row 2) to enter layer-selection mode.

While FN is held, **K1–K6 select layers 0–5**, and **row 2 becomes Tab / Space / Enter**:

```
┌────────┬────────┬────────┬────────┐
│ Lyr 0  │ Lyr 1  │ Lyr 2  │ Lyr 3  │  ← Row 0: K1-K4
├────────┼────────┼────────┼────────┤
│ Lyr 4  │ Lyr 5  │   —    │   —    │  ← Row 1: K5-K6 (K7-K8 inactive)
├────────┼────────┼────────┼────────┤
│ (hold) │  Tab   │ Space  │ Enter  │  ← Row 2: FN + Tab/Space/Enter
└────────┴────────┴────────┴────────┘
```

**Behaviour:**
- All regular key output is suppressed while FN is held.
- The OLED shows **"FN / select:"** while waiting.
- Pressing a layer key switches to that layer and shows a **brief preview** of the new layer's name and key hints (~1.5 s), then returns to the FN overlay.
- Tab, Space, and Enter are available at row 2 as normal key output while FN is held.
- Releasing FN confirms the selection and shows the current layer info.

---

## Rotary Encoder

The rotary encoder button **cycles through all 6 layers in order** (0→1→2→3→4→5→0).

| Layer | Knob CCW | Knob CW |
|-------|----------|---------|
| 0 (VOL) | Volume Down | Volume Up |
| 1 (SCR) | Scroll Down | Scroll Up |
| 2 (BRT) | Brightness Down | Brightness Up |
| 3–5 | *(unbound — shows warning)* | *(unbound — shows warning)* |

Turning the rotary on an **unbound layer** (3–5) shows a brief **"ROTARY / no bind"** message on the OLED instead of sending a keycode or triggering the comet LED effect.

---

## Joystick

| Action | Function |
|--------|----------|
| Tilt in any direction | Moves the mouse cursor (analog, proportional) |
| Press (click) | Launches the built-in mini-game (koebiten) |

> The game takes over the display and rotary encoder. Power-cycle or reset to return to normal keyboard mode.

---

## RGB LEDs

12 WS2812B LEDs run a continuous **rainbow animation** at idle, with each LED offset in hue for a flowing effect.

When a key is pressed, the corresponding LED **flashes bright white** and fades back to rainbow over ~200 ms. The LED index is correctly mapped from the row-major key matrix to the column-major LED wiring, so the lit LED always matches the physical key.

When the **rotary encoder turns**, the LEDs switch to a **white comet** on the outer ring: a bright head with two fading trail LEDs chasing the direction of rotation.

---

## OLED Display

| State | Trigger | Content | Duration |
|-------|---------|---------|----------|
| Layer info | Layer change / FN release | Layer name + key hints | 3.2 s, then screensaver |
| FN overlay | FN held | `FN` + `select:` | Until key pressed or FN released |
| FN preview | FN held + layer selected | New layer name + key hints | 1.5 s, then back to FN overlay |
| Shift hint | Shift key held | `SHIFT` + uppercased key hints for current layer | Until Shift released |
| Rotary warning | Rotary turned on unbound layer | `ROTARY` + `no bind` | ~1.5 s, then layer info |
| Screensaver | Idle | Bouncing pixel | Until next event |

Key hint format: `xxxx Syyy` where `S` = Shift, `x`/`y` = letter/digit/symbol. Navigation layer shows `H^EU <v>D` (H=Home, ^=Up, E=End, U=PgUp, <=Left, v=Down, >=Right, D=PgDn).

---

## Vial Customisation

Connect the device while Vial is open — all 6 layers are visible and editable live. Changes take effect immediately and are saved to flash. **Note:** firmware defaults are re-applied on each boot to ensure compatibility after firmware upgrades; Vial edits are session-persistent but reset on power cycle or reflash.

All key assignments live in [`firmware/layers.go`](firmware/layers.go). The `layerKeys` array holds firmware defaults for all six layers (12 keys each, row-major order).

### Changing a key

Edit `layerKeys` in `firmware/layers.go`:

```go
LayerAG: {
    jp.KeyA, jp.KeyB, jp.KeyC, jp.KeyTab,  // changed K4 from 'd' to Tab
    jp.KeyLeftShift, jp.KeyE, jp.KeyF, jp.KeyG,
    0, jp.KeyBackspace, jp.MouseLeft, jp.MouseRight,
},
```

### Available keycodes

```go
jp.KeyA … jp.KeyZ                               // letters
jp.Key1 … jp.Key0                               // digits
jp.KeyLeftShift, jp.KeyRightShift               // modifier keys
jp.KeySpace, jp.KeyEnter, jp.KeyBackspace, jp.KeyDelete
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
