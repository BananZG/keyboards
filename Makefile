# ============================================================================
# Keyboard Configuration Database
# ============================================================================
KEYBOARDS := sg24-left sg24-right zero-kb02 zero-kb02-invert panel25 sg48key2 conf2025badge conf2025badge-sound

# Configuration format: NAME_target, NAME_tags, NAME_path, NAME_out

sg24-left_target := waveshare-rp2040-zero
sg24-left_tags :=
sg24-left_path := ./sg24/firmware/left/
sg24-left_out := ./out/sg24-left.uf2

sg24-right_target := waveshare-rp2040-zero
sg24-right_tags :=
sg24-right_path := ./sg24/firmware/right/
sg24-right_out := ./out/sg24-right.uf2

zero-kb02_target := waveshare-rp2040-zero
zero-kb02_tags := zero_kb02
zero-kb02_path := ./zero-kb02/firmware/
zero-kb02_out := ./out/zero-kb02.uf2

zero-kb02-invert_target := waveshare-rp2040-zero
zero-kb02-invert_tags := zero_kb02,invert_rotary_pins
zero-kb02-invert_path := ./zero-kb02/firmware/
zero-kb02-invert_out := ./out/zero-kb02-invert.uf2

panel25_target := waveshare-rp2040-zero
panel25_tags :=
panel25_path := ./panel25/firmware/
panel25_out := ./out/panel25.uf2

sg48key2_target := waveshare-rp2040-zero
sg48key2_tags :=
sg48key2_path := ./sg48key2/firmware/
sg48key2_out := ./out/sg48key2.uf2

conf2025badge_target := xiao-rp2040
conf2025badge_tags := conf2025badge
conf2025badge_path := ./conf2025badge/firmware/
conf2025badge_out := ./out/conf2025badge.uf2

conf2025badge-sound_target := xiao-rp2040
conf2025badge-sound_tags := conf2025badge,with_sound
conf2025badge-sound_path := ./conf2025badge/firmware/
conf2025badge-sound_out := ./out/conf2025badge_sound.uf2

# ============================================================================
# Helper functions
# ============================================================================

# Validate KEY variable and check if keyboard exists
validate-key:
	@if [ -z "$(KEY)" ]; then \
		echo "Error: KEY not specified"; \
		echo "Available keyboards: $(KEYBOARDS)"; \
		exit 1; \
	fi
	@if [ -z "$($(KEY)_target)" ]; then \
		echo "Error: Unknown keyboard '$(KEY)'"; \
		echo "Available keyboards: $(KEYBOARDS)"; \
		exit 1; \
	fi

# ============================================================================
# Build targets
# ============================================================================

.PHONY: build
build: validate-key
	@mkdir -p out
	tinygo build -o $($(KEY)_out) --target $($(KEY)_target) --size short --stack-size 8kb $(if $($(KEY)_tags),--tags $($(KEY)_tags)) $($(KEY)_path)

.PHONY: flash
flash: validate-key
	tinygo flash --target $($(KEY)_target) --size short --stack-size 8kb $(if $($(KEY)_tags),--tags $($(KEY)_tags)) $($(KEY)_path)

.PHONY: build-flash
build-flash: build flash

.PHONY: smoketest
smoketest: FORCE
	@mkdir -p out
	@echo "Running smoketest for all keyboards..."
	@for kb in $(KEYBOARDS); do \
		$(MAKE) build KEY=$$kb || exit 1; \
	done
	@echo "✓ All keyboards built successfully"

# ============================================================================
# Utility targets
# ============================================================================

FORCE:

.PHONY: gen-def-with-find
gen-def-with-find:
	find . -name vial.json | xargs -n 1 go run github.com/sago35/tinygo-keyboard/cmd/gen-def

.PHONY: list-keyboards
list-keyboards:
	@echo "Available keyboards:"
	@for kb in $(KEYBOARDS); do \
		echo "  - $$kb"; \
	done
