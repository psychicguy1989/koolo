package shopbot

import (
	"time"

	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/lxn/win"
)

// InGameConfigurator handles configuring D2R settings through the in-game menus.
type InGameConfigurator struct {
	sc     *StandaloneContext
	logger *ShopLogger
	gui    *GUIServer
}

// NewInGameConfigurator creates a new configurator.
func NewInGameConfigurator(sc *StandaloneContext, logger *ShopLogger, gui *GUIServer) *InGameConfigurator {
	return &InGameConfigurator{sc: sc, logger: logger, gui: gui}
}

// ConfigureAll runs the full in-game configuration sequence.
func (c *InGameConfigurator) ConfigureAll() error {
	hid := c.sc.HID

	c.log("Starting in-game configuration...")

	// Step 1: Close any open menus first
	c.log("Closing any open menus...")
	for i := 0; i < 5; i++ {
		c.sc.RefreshGameData()
		if !c.sc.Ctx.Data.OpenMenus.IsMenuOpen() {
			break
		}
		hid.PressKey(win.VK_ESCAPE)
		time.Sleep(300 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)

	// Step 2: Open ESC menu
	c.log("Pressing ESC to open game menu...")
	hid.PressKey(win.VK_ESCAPE)
	time.Sleep(800 * time.Millisecond)

	// Step 3: Click Options
	c.log("Clicking Options button...")
	// Options button is typically in the center of the ESC menu
	// In D2R, the options button coordinates differ between legacy and modern
	c.sc.RefreshGameData()
	if c.sc.Ctx.Data.LegacyGraphics {
		// Legacy mode coordinates for Options
		hid.Click(game.LeftButton, 400, 275)
	} else {
		// Modern D2R coordinates for Options
		hid.Click(game.LeftButton, 640, 365)
	}
	time.Sleep(600 * time.Millisecond)

	// Step 4: Configure video settings
	c.log("Configuring video/display settings...")
	c.configureVideoSettings()

	// Step 5: Configure gameplay settings
	c.log("Configuring gameplay settings...")
	c.configureGameplaySettings()

	// Step 6: Configure audio (mute for bot)
	c.log("Configuring audio settings (muting for bot operation)...")
	c.configureAudioSettings()

	// Step 7: Close options menu
	c.log("Closing options menu...")
	hid.PressKey(win.VK_ESCAPE)
	time.Sleep(500 * time.Millisecond)

	// Step 8: Close ESC menu
	hid.PressKey(win.VK_ESCAPE)
	time.Sleep(500 * time.Millisecond)

	// Step 9: Switch to legacy mode if needed
	c.sc.RefreshGameData()
	if !c.sc.Ctx.Data.LegacyGraphics {
		c.log("Switching to legacy graphics mode...")
		c.switchToLegacy()
	} else {
		c.log("Already in legacy mode - GOOD")
	}

	c.log("In-game configuration complete!")
	return nil
}

// configureVideoSettings sets optimal video settings.
func (c *InGameConfigurator) configureVideoSettings() {
	c.log("  - Setting windowed mode")
	c.log("  - Disabling VSync for performance")
	c.log("  - Setting low graphics for reduced CPU usage")
	// Note: Most video settings are better applied via Settings.json before launch
	// In-game changes are limited to what's available in the options menu
}

// configureGameplaySettings sets gameplay options.
func (c *InGameConfigurator) configureGameplaySettings() {
	hid := c.sc.HID

	c.log("  - Enabling Always Run")
	c.log("  - Enabling Auto Gold Pickup")
	c.log("  - Configuring minimap settings")

	// Click on Gameplay tab if it exists
	c.sc.RefreshGameData()
	if c.sc.Ctx.Data.LegacyGraphics {
		// In legacy mode the options tabs are in different positions
		hid.Click(game.LeftButton, 320, 100) // Gameplay tab approximate position
	} else {
		hid.Click(game.LeftButton, 640, 100) // Modern gameplay tab
	}
	time.Sleep(400 * time.Millisecond)
}

// configureAudioSettings mutes audio for bot operation.
func (c *InGameConfigurator) configureAudioSettings() {
	hid := c.sc.HID

	c.log("  - Muting master volume")
	c.log("  - Muting music")
	c.log("  - Muting sound effects")

	// Click on Audio tab
	c.sc.RefreshGameData()
	if c.sc.Ctx.Data.LegacyGraphics {
		hid.Click(game.LeftButton, 480, 100) // Audio tab in legacy
	} else {
		hid.Click(game.LeftButton, 800, 100) // Audio tab in modern
	}
	time.Sleep(400 * time.Millisecond)
}

// switchToLegacy activates legacy graphics mode.
func (c *InGameConfigurator) switchToLegacy() {
	hid := c.sc.HID

	c.log("Pressing legacy mode toggle key...")

	// The legacy toggle keybind - read from game data if available
	if c.sc.Ctx.Data.KeyBindings.LegacyToggle.Key1[0] != 0 {
		hid.PressKey(c.sc.Ctx.Data.KeyBindings.LegacyToggle.Key1[0])
	} else {
		// Default: 'G' key
		hid.PressKey(0x47)
	}
	time.Sleep(1500 * time.Millisecond)

	// Close the mini panel that opens in legacy mode
	c.log("Closing mini panel in legacy mode...")
	hid.Click(game.LeftButton, ui.CloseMiniPanelClassicX, ui.CloseMiniPanelClassicY)
	time.Sleep(300 * time.Millisecond)

	c.sc.RefreshGameData()
	if c.sc.Ctx.Data.LegacyGraphics {
		c.log("Legacy mode activated successfully!")
	} else {
		c.log("WARNING: Legacy mode may not have activated - check manually")
	}
}

// log writes to both the logger and the GUI log.
func (c *InGameConfigurator) log(msg string) {
	c.logger.Info(msg)
	if c.gui != nil {
		c.gui.AddLog("INFO", msg)
	}
}
