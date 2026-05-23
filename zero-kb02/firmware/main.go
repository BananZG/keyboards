package main

import (
	"log"
	"machine"
	"machine/usb"
	"machine/usb/hid/mouse"
	"runtime/interrupt"
	"runtime/volatile"
	"time"

	"github.com/sago35/koebiten"
	"github.com/sago35/koebiten/games/all/all"
	"github.com/sago35/koebiten/hardware"
	keyboard "github.com/sago35/tinygo-keyboard"
	jp "github.com/sago35/tinygo-keyboard/keycodes/japanese"
	pio "github.com/tinygo-org/pio/rp2-pio"
	"github.com/tinygo-org/pio/rp2-pio/piolib"
	"tinygo.org/x/drivers"
	"tinygo.org/x/drivers/ssd1306"
)

// ---- Main ----

func main() {
	usb.Product = "zero-kb02-0.1.0"
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// --- I2C / Display setup ---
	i2c := machine.I2C0
	i2c.Configure(machine.I2CConfig{
		Frequency: 2.8 * machine.MHz,
		SDA:       machine.GPIO12,
		SCL:       machine.GPIO13,
	})

	display := ssd1306.NewI2C(i2c)
	display.Configure(ssd1306.Config{
		Address:  0x3C,
		Width:    128,
		Height:   64,
		Rotation: drivers.Rotation180,
	})
	display.ClearDisplay()

	displayBuf := NewDisplayBuffer(display.Size())

	// --- WS2812B LED setup ---
	s, _ := pio.PIO0.ClaimStateMachine()
	ws, _ := piolib.NewWS2812B(s, machine.GPIO1)
	if err := ws.EnableDMA(true); err != nil {
		return err
	}

	// wsLeds holds raw GRB values for all 12 LEDs.
	// Pre-allocated here, never reallocated.
	var wsLeds [12]uint32
	var wsFlash [12]uint8 // flash countdown per key; each tick = 32 ms
	var rainbowHue uint8

	// --- ADC (joystick) ---
	machine.InitADC()
	m := mouse.Port()
	x := NewADCDevice(machine.GPIO29, 0x3000, 0xC800, false)
	y := NewADCDevice(machine.GPIO28, 0x3000, 0xC800, true)

	// --- Keyboard driver ---
	d := keyboard.New()

	colPins := []machine.Pin{machine.GPIO5, machine.GPIO6, machine.GPIO7, machine.GPIO8}
	rowPins := []machine.Pin{machine.GPIO9, machine.GPIO10, machine.GPIO11}

	// Page manager - allocated on stack, no heap
	var pm PageManager
	pm.Init()

	// changed signals the display loop that LEDs/display need refreshing
	var changed volatile.Register8

	// ---- Matrix keyboard ----
	// The tinygo-keyboard library calls our callback with the *vial layer*
	// index (0-2) and the *physical key index* (0-11, row-major).
	// We use the library's layer mechanism only for the rotary knob.
	// For the matrix we always register on layer 0 and drive our own page
	// logic here.
	//
	// Physical index layout (row-major, 4 cols × 3 rows):
	//   Row 0: idx 0-3
	//   Row 1: idx 4-7
	//   Row 2: idx 8-11  →  FN(8) | Backspace(9) | MouseL(10) | MouseR(11)
	//
	// We give the matrix a single layer of placeholder codes; actual output
	// is controlled via the callback + pm.EffectivePage().
	// Build initial matrix layer: rows 0-1 start on PageLowerA, row 2 is fixed.
	// d.SetKeycode(layer, kbIndex, index, key) updates rows 0-1 on page change.
	var initialLayer [12]keyboard.Keycode
	for row := 0; row < 2; row++ {
		for col := 0; col < 4; col++ {
			initialLayer[row*4+col] = keyPages[PageLowerA][row][col]
		}
	}
	initialLayer[8] = 0               // FN key: no HID code, handled in callback
	initialLayer[9] = jp.KeyBackspace // Delete/Backspace
	initialLayer[10] = jp.MouseLeft   // Mouse left click
	initialLayer[11] = jp.MouseRight  // Mouse right click

	mk := d.AddMatrixKeyboard(colPins, rowPins, [][]keyboard.Keycode{initialLayer[:]})
	// AddMatrixKeyboard only populates layer 0; prime layers 1-2 now so the
	// keyboard works in all rotary modes from startup (before any FN press).
	fillLayerKeys(d, PageLowerA)
	setMatrixKey(d, 9, jp.KeyBackspace)
	setMatrixKey(d, 10, jp.MouseLeft)
	setMatrixKey(d, 11, jp.MouseRight)

	mk.SetCallback(func(layer, index int, state keyboard.State) {
		row := index / 4
		col := index % 4

		// ---- FN key (row 2, col 0) ----
		// When held: zero rows 0-1 so no letters fire.
		// When released: restore current page keycodes.
		if row == 2 && col == 0 {
			if state == keyboard.Press {
				pm.SetFN(true)
				// Zero ALL matrix keycodes on every rotary layer while FN is
				// held so no keys fire regardless of the active rotary layer.
				for i := 0; i < 12; i++ {
					setMatrixKey(d, i, 0)
				}
			} else if state == keyboard.PressToRelease {
				pm.SetFN(false)
				fillLayerKeys(d, int(pm.currentPage))
				// Restore fixed row-2 non-FN keys on all rotary layers
				setMatrixKey(d, 9, jp.KeyBackspace)
				setMatrixKey(d, 10, jp.MouseLeft)
				setMatrixKey(d, 11, jp.MouseRight)
			}
			changed.Set(1)
			return
		}

		// ---- FN held + key press → page selection (all rows) ----
		// fnPageKeys returns -1 for the FN key itself (row2,col0), so it is safe
		// to remove the row<2 guard that was previously preventing row-3 shortcuts.
		if pm.fnPressed && state == keyboard.Press {
			if target := GetFNPageTarget(row, col); target >= 0 {
				pm.SetPage(target)
				// Keycodes restored to the new page when FN is released.
			}
			changed.Set(1)
			return
		}

		// ---- LED flash on key press ----
		// Key sending for rows 0-1 and fixed row-2 keys is handled by the library
		// based on the keycodes registered via SetKeycode / AddMatrixKeyboard.
		if state == keyboard.Press {
			mask := interrupt.Disable()
			wsFlash[index] = 6 // 6 × 32 ms ≈ 192 ms fade
			interrupt.Restore(mask)
			changed.Set(1)
		}
	})

	// MouseLeft (index 10) and MouseRight (index 11) are part of the matrix.
	// jp.MouseLeft / jp.MouseRight keycodes handle clicks via the library.

	// ---- Rotary encoder (unchanged from original) ----
	rotaryPins := [2]machine.Pin{machine.GPIO3, machine.GPIO4}
	if invertRotaryPins {
		rotaryPins[0], rotaryPins[1] = rotaryPins[1], rotaryPins[0]
	}
	rk := d.AddRotaryKeyboard(rotaryPins[0], rotaryPins[1], [][]keyboard.Keycode{
		{jp.KeyMediaVolumeDec, jp.KeyMediaVolumeInc},         // layer 0: volume
		{jp.KeyLeft, jp.KeyRight},                            // layer 1: cursor
		{jp.KeyMediaBrightnessDown, jp.KeyMediaBrightnessUp}, // layer 2: brightness
	})
	// When the rotary turns, show a white comet chasing the direction.
	// index 0 = CCW (decrease), index 1 = CW (increase).
	rk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.Press {
			dir := 1
			if index == 0 {
				dir = -1
			}
			RotarySpinAdvance(dir)
		}
	})

	gpioPins := []machine.Pin{machine.GPIO0, machine.GPIO2}
	for i := range gpioPins {
		gpioPins[i].Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	var koebitenEnable bool // set true by gk callback, consumed in main loop
	gk := d.AddGpioKeyboard(gpioPins, [][]keyboard.Keycode{
		{jp.KeyTo5, jp.KeyTo1},
		{jp.KeyTo5, jp.KeyTo2},
		{jp.KeyTo5, jp.KeyTo0},
	})
	// On joystick press: library sends KeyTo5, moving to layer 5.
	// On release at layer 5: signal game launch to the main loop.
	gk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.PressToRelease && d.Layer() == 5 {
			koebitenEnable = true
		}
	})

	loadKeyboardDef()
	d.Init()

	// ---- State ----
	var (
		displayMode  = showPageInfo
		displayTimer = 0
		rotaryLayer  = 0 // mirrors d.Layer() to detect rotary layer changes
	)

	dispx, dispy := int16(0), int16(0)
	deltaX, deltaY := int16(1), int16(1)

	cnt := 0

	for {
		time.Sleep(1 * time.Millisecond)

		if err := d.Tick(); err != nil {
			return err
		}

		// Launch koebiten game when joystick activated it
		if koebitenEnable {
			koebitenEnable = false
			// Detach rotary encoder GPIO interrupts so the game framework can use them.
			machine.GPIO3.SetInterrupt(machine.PinToggle, nil)
			machine.GPIO4.SetInterrupt(machine.PinToggle, nil)
			koebiten.SetHardware(hardware.Device)
			koebiten.SetRotation(koebiten.Rotation0)
			game := all.NewGame()
			if err := koebiten.RunGame(game); err != nil {
				log.Fatal(err)
			}
			game.RunCurrentGame()
		}

		// Detect rotary layer changes (GPIO keyboard sends KeyTo0/1/2/5)
		if cur := d.Layer(); cur != rotaryLayer {
			rotaryLayer = cur
			if cur < 5 { // layer 5 is game mode; handled above
				displayMode = showLayerInfo
				displayTimer = 0
				renderLayerInfo(display, cur)
			}
		}

		// Joystick mouse movement (every 10ms)
		if cnt%10 == 0 {
			m.Move(int(x.Get2()), int(y.Get2()))
		}

		// LED update (every 32ms)
		if cnt%32 == 0 {
			mask := interrupt.Disable()
			if wsSpinTimer > 0 {
				// Rotary spin mode: dark background, white comet on outer ring.
				// Head (0x60) + two fading trail LEDs (0x18, 0x06); inner LEDs off.
				wsSpinTimer--
				for i := range wsLeds {
					wsLeds[i] = 0
				}
				for behind := 2; behind >= 0; behind-- {
					ringPos := (int(wsSpinPos) - behind + 10) % 10
					ledIdx := outerRing[ringPos]
					switch behind {
					case 0:
						wsLeds[ledIdx] = 0x60606000 // head
					case 1:
						wsLeds[ledIdx] = 0x18181800 // trail 1
					case 2:
						wsLeds[ledIdx] = 0x06060600 // trail 2
					}
				}
				// Key-press flash always wins over the comet
				for i, f := range wsFlash {
					if f > 0 {
						bright := uint32(f) * 16
						wsLeds[i] = (bright << 24) | (bright << 16) | (bright << 8)
						wsFlash[i]--
					}
				}
			} else {
				// Rainbow mode: continuous hue rotation, flash on key press
				for i := range wsLeds {
					if wsFlash[i] > 0 {
						bright := uint32(wsFlash[i]) * 16
						wsFlash[i]--
						wsLeds[i] = (bright << 24) | (bright << 16) | (bright << 8)
					} else {
						wsLeds[i] = hsvToGRB(rainbowHue + uint8(i*22))
					}
				}
				rainbowHue++
			}
			ws.WriteRaw(wsLeds[:])
			interrupt.Restore(mask)
		}

		// Display update (every 16ms)
		if cnt%16 == 0 {
			if pm.ConsumeDisplayUpdate() {
				if pm.fnPressed {
					switch displayMode {
					case showFNOverlay, showFNPreview:
						// Page selected while FN held → preview new page name
						renderPageInfo(display, displayBuf, int(pm.currentPage))
						displayMode = showFNPreview
						displayTimer = 0
					default:
						// FN key just pressed → show FN overlay
						renderFNInfo(display)
						displayMode = showFNOverlay
					}
				} else {
					// FN released or normal page change
					renderPageInfo(display, displayBuf, pm.EffectivePage())
					displayMode = showPageInfo
					displayTimer = 0
				}
			}

			switch displayMode {
			case showPageInfo:
				displayTimer++
				if displayTimer > 200 { // ~3.2 s
					displayMode = showScreensaver
				}
			case showLayerInfo:
				displayTimer++
				if displayTimer > 100 { // ~1.6 s
					displayMode = showPageInfo
					renderPageInfo(display, displayBuf, pm.EffectivePage())
					displayTimer = 0
				}
			case showFNOverlay:
				// Stays until ConsumeDisplayUpdate fires (page selected or FN released)
			case showFNPreview:
				displayTimer++
				if displayTimer > 96 { // ~1.5 s
					renderFNInfo(display)
					displayMode = showFNOverlay
					displayTimer = 0
				}
			case showScreensaver:
				renderScreensaver(display, displayBuf, &dispx, &dispy, &deltaX, &deltaY)
			}
		}

		cnt++
		if cnt >= 1000 {
			cnt = 0
		}
	}
}

// setMatrixKey sets a matrix keycode on all three rotary layers (0-2) so the
// key works regardless of which rotary encoder mode is currently active.
func setMatrixKey(d *keyboard.Device, idx int, key keyboard.Keycode) {
	d.SetKeycode(0, 0, idx, key)
	d.SetKeycode(1, 0, idx, key)
	d.SetKeycode(2, 0, idx, key)
}

// fillLayerKeys updates rows 0-1 of the matrix for the given page across all
// rotary layers so typing works in every rotary encoder mode.
func fillLayerKeys(d *keyboard.Device, page int) {
	for row := 0; row < 2; row++ {
		for col := 0; col < 4; col++ {
			setMatrixKey(d, row*4+col, keyPages[page][row][col])
		}
	}
}

// ---- Package-level vars kept to minimum ----

var invertRotaryPins = false
