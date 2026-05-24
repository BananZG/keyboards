package main

import (
	"log"
	"machine"
	"machine/usb"
	"machine/usb/hid/mouse"
	"runtime/interrupt"
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

	// ---- Matrix keyboard ----
	// All six Vial layers are registered with firmware-default keycodes.
	// After d.Init() the defaults are re-applied to overwrite any stale flash.
	//
	// Physical index layout (row-major, 4 cols × 3 rows):
	//   Row 0 (idx 0-3):  K1 K2 K3 K4
	//   Row 1 (idx 4-7):  Sft K6 K7 K8   (Shift at index 4 on layers 0-3)
	//   Row 2 (idx 8-11): FN(8) Del(9) MouseL(10) MouseR(11)
	mk := d.AddMatrixKeyboard(colPins, rowPins, [][]keyboard.Keycode{
		layerKeys[0][:],
		layerKeys[1][:],
		layerKeys[2][:],
		layerKeys[3][:],
		layerKeys[4][:],
		layerKeys[5][:],
	})

	// fnKeyCache saves all six layers × 12 keys during FN hold so that
	// Vial-configured keycodes survive a press-release cycle unchanged.
	// Stack cost: 6 × 12 × 2 = 144 bytes.
	var fnKeyCache [keyboard.LayerCount][12]keyboard.Keycode
	var fnActive bool             // true while FN key is held
	var fnPreviewLayer = int8(-1) // layer being previewed during FN hold, -1 = none
	var displayUpdate = true      // trigger initial layer display on startup
	var shiftHeld bool            // true while Shift is physically held
	var rotaryWarning bool        // set when rotary turned on an unbound layer

	mk.SetCallback(func(layer, index int, state keyboard.State) {
		// ---- FN key (row 2, col 0 = index 8) ----
		if index == 8 {
			if state == keyboard.Press {
				fnActive = true
				fnPreviewLayer = -1
				displayUpdate = true
				// Cache all six layers so Vial keycodes can be restored on release.
				for l := 0; l < keyboard.LayerCount; l++ {
					for i := 0; i < 12; i++ {
						fnKeyCache[l][i] = mk.Key(l, i)
					}
				}
				// Install FN overlay: K1-K6 (idx 0-5) switch layers 0-5;
				// K7-K8 (idx 6-7) suppressed; row-2 (idx 9-11) get Tab/Space/Enter.
				for l := 0; l < keyboard.LayerCount; l++ {
					d.SetKeycode(l, 0, 0, jp.KeyTo0)
					d.SetKeycode(l, 0, 1, jp.KeyTo1)
					d.SetKeycode(l, 0, 2, jp.KeyTo2)
					d.SetKeycode(l, 0, 3, jp.KeyTo3)
					d.SetKeycode(l, 0, 4, jp.KeyTo4)
					d.SetKeycode(l, 0, 5, jp.KeyTo5)
					d.SetKeycode(l, 0, 6, 0)
					d.SetKeycode(l, 0, 7, 0)
					d.SetKeycode(l, 0, 9, fnRow2Keys[0])
					d.SetKeycode(l, 0, 10, fnRow2Keys[1])
					d.SetKeycode(l, 0, 11, fnRow2Keys[2])
				}
			} else if state == keyboard.PressToRelease {
				fnActive = false
				fnPreviewLayer = -1
				displayUpdate = true
				// Restore all six layers from cache.
				for l := 0; l < keyboard.LayerCount; l++ {
					for i := 0; i < 12; i++ {
						d.SetKeycode(l, 0, i, fnKeyCache[l][i])
					}
				}
			}
			return
		}

		// Track which layer the user selected while FN is held.
		// KeyTo0-5 are installed at indices 0-5, so index == target layer.
		if fnActive && state == keyboard.Press && index < keyboard.LayerCount {
			fnPreviewLayer = int8(index)
			displayUpdate = true
		}

		// Detect Shift press/release for the screen hint (only outside FN mode).
		if !fnActive {
			kc := mk.Key(layer, index)
			if kc == jp.KeyLeftShift || kc == jp.KeyRightShift {
				if state == keyboard.Press {
					shiftHeld = true
					displayUpdate = true
				} else if state == keyboard.PressToRelease {
					shiftHeld = false
					displayUpdate = true
				}
			}
		}

		// LED flash on key press.
		// Matrix keys use row-major index (row*4+col) but LEDs are wired
		// column-major (col0=0-2, col1=3-5, col2=6-8, col3=9-11), so convert.
		if state == keyboard.Press {
			ledIdx := (index%4)*3 + index/4
			mask := interrupt.Disable()
			wsFlash[ledIdx] = 6 // 6 × 32 ms ≈ 192 ms fade
			interrupt.Restore(mask)
		}
	})

	// ---- Rotary encoder ----
	// Layers 0-2 define the rotary mode; layers 3-5 default to volume.
	rotaryPins := [2]machine.Pin{machine.GPIO3, machine.GPIO4}
	if invertRotaryPins {
		rotaryPins[0], rotaryPins[1] = rotaryPins[1], rotaryPins[0]
	}
	rk := d.AddRotaryKeyboard(rotaryPins[0], rotaryPins[1], [][]keyboard.Keycode{
		rkLayerKeys[0][:],
		rkLayerKeys[1][:],
		rkLayerKeys[2][:],
		rkLayerKeys[3][:],
		rkLayerKeys[4][:],
		rkLayerKeys[5][:],
	})
	// Comet effect only on layers with a named rotary action (layers 0-2).
	// On unbound layers a brief warning is shown on the OLED instead.
	rk.SetCallback(func(layer, index int, state keyboard.State) {
		if state == keyboard.Press {
			if layerRotaryLabel[layer] != "" {
				dir := 1
				if index == 0 {
					dir = -1
				}
				RotarySpinAdvance(dir)
			} else {
				rotaryWarning = true
				displayUpdate = true
			}
		}
	})

	gpioPins := []machine.Pin{machine.GPIO0, machine.GPIO2}
	for _, p := range gpioPins {
		p.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	}
	var koebitenEnable bool
	// Rotary button (index 1): cycles through all 6 layers in order (0→1→2→...→0).
	// Joystick (index 0): no keycode — game mode is triggered by the callback.
	gk := d.AddGpioKeyboard(gpioPins, [][]keyboard.Keycode{
		gkLayerKeys[0][:],
		gkLayerKeys[1][:],
		gkLayerKeys[2][:],
		gkLayerKeys[3][:],
		gkLayerKeys[4][:],
		gkLayerKeys[5][:],
	})
	// Joystick press (index 0, PressToRelease) triggers game mode directly,
	// independent of the active Vial layer.
	gk.SetCallback(func(layer, index int, state keyboard.State) {
		if index == 0 && state == keyboard.PressToRelease {
			koebitenEnable = true
		}
	})

	loadKeyboardDef()
	d.Init()
	// Re-apply firmware defaults after Init() so stale flash from a previous
	// firmware version is immediately overwritten for all three keyboard devices.
	for l := 0; l < keyboard.LayerCount; l++ {
		for i := 0; i < 12; i++ {
			d.SetKeycode(l, 0, i, layerKeys[l][i]) // matrix  (kbIndex 0)
		}
		d.SetKeycode(l, 1, 0, rkLayerKeys[l][0]) // rotary  (kbIndex 1)
		d.SetKeycode(l, 1, 1, rkLayerKeys[l][1])
		d.SetKeycode(l, 2, 0, gkLayerKeys[l][0]) // GPIO    (kbIndex 2)
		d.SetKeycode(l, 2, 1, gkLayerKeys[l][1])
	}

	// ---- State ----
	var (
		displayMode  = showPageInfo
		displayTimer = 0
		currentLayer = 0 // mirrors d.Layer() to detect changes
	)

	ss := screensaverState{dx: 1, dy: 1}

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

		// Detect layer changes (rotary button cycles layers 0-2).
		// FN-triggered changes are handled by the mk callback directly.
		if cur := d.Layer(); cur != currentLayer {
			currentLayer = cur
			if !fnActive {
				displayMode = showPageInfo
				displayUpdate = true
				displayTimer = 0
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

		// Display update (every 16 ms)
		if cnt%16 == 0 {
			if displayUpdate {
				displayUpdate = false
				switch {
				case fnActive && fnPreviewLayer >= 0:
					// FN held + layer key pressed: preview selected layer.
					renderLayerInfo(display, displayBuf, int(fnPreviewLayer))
					displayMode = showFNPreview
					displayTimer = 0
				case fnActive:
					// FN just pressed: show overlay.
					renderFNInfo(display)
					displayMode = showFNOverlay
				case shiftHeld:
					// Shift held: show uppercased key hints.
					renderShiftHint(display, displayBuf, currentLayer)
					displayMode = showShiftHint
				case rotaryWarning:
					// Rotary turned on unbound layer: show brief warning.
					rotaryWarning = false
					renderRotaryWarning(display)
					displayMode = showRotaryWarning
					displayTimer = 0
				default:
					// Normal mode: show current layer.
					renderLayerInfo(display, displayBuf, currentLayer)
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
			case showShiftHint:
				// Stays until Shift release triggers displayUpdate.
			case showRotaryWarning:
				displayTimer++
				if displayTimer > 96 { // ~1.5 s then back to layer info
					renderLayerInfo(display, displayBuf, currentLayer)
					displayMode = showPageInfo
					displayTimer = 0
				}
			case showFNOverlay:
				// Stays until displayUpdate fires (layer selected or FN released).
			case showFNPreview:
				displayTimer++
				if displayTimer > 96 { // ~1.5 s
					renderFNInfo(display)
					displayMode = showFNOverlay
					fnPreviewLayer = -1
					displayTimer = 0
				}
			case showScreensaver:
				renderScreensaver(display, displayBuf, &ss)
			}
		}

		cnt++
		if cnt >= 160 { // LCM(10, 16, 32): all three update intervals divide evenly
			cnt = 0
		}
	}
}

// ---- Package-level vars kept to minimum ----

const invertRotaryPins = false
