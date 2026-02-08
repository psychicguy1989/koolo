package shopbot

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// D2RSettings represents the D2R Settings.json structure for auto-configuration.
type D2RSettings map[string]interface{}

// Launcher handles starting D2R and configuring settings automatically.
type Launcher struct {
	cfg    *ShopBotConfig
	logger *ShopLogger
}

// NewLauncher creates a new D2R launcher.
func NewLauncher(cfg *ShopBotConfig, logger *ShopLogger) *Launcher {
	return &Launcher{cfg: cfg, logger: logger}
}

// EnsureD2RRunning checks if D2R is running and starts it if auto_start is enabled.
func (l *Launcher) EnsureD2RRunning() error {
	if !l.cfg.AutoStartD2R {
		l.logger.Info("Auto-start disabled, expecting D2R to be running already")
		return nil
	}

	// Check if already running
	if l.isD2RRunning() {
		l.logger.Info("D2R is already running")
		return nil
	}

	l.logger.Info("Starting D2R...")

	// Apply settings before launch if configured
	if l.cfg.AutoConfigureD2R {
		if err := l.applyOptimalSettings(); err != nil {
			l.logger.Warn("Failed to apply settings (non-fatal)", slog.String("error", err.Error()))
		}
	}

	// Launch D2R
	return l.startD2R()
}

// applyOptimalSettings writes optimized Settings.json for shop bot usage.
func (l *Launcher) applyOptimalSettings() error {
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return fmt.Errorf("USERPROFILE environment variable not set")
	}

	settingsDir := filepath.Join(userProfile, "Saved Games", "Diablo II Resurrected")
	settingsPath := filepath.Join(settingsDir, "Settings.json")

	// Read existing settings or create new
	settings := l.getOptimalSettings()

	// Backup existing settings
	if _, err := os.Stat(settingsPath); err == nil {
		backupPath := settingsPath + ".shopbot.bak"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			data, readErr := os.ReadFile(settingsPath)
			if readErr == nil {
				os.WriteFile(backupPath, data, 0644)
				l.logger.Info("Backed up existing settings", slog.String("path", backupPath))
			}
		}
	}

	// Write optimal settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling settings: %w", err)
	}

	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("writing settings: %w", err)
	}

	l.logger.Info("Applied optimal D2R settings",
		slog.String("path", settingsPath),
	)

	return nil
}

// getOptimalSettings returns D2R settings optimized for the shop bot.
func (l *Launcher) getOptimalSettings() D2RSettings {
	return D2RSettings{
		// Graphics - optimized for performance and legacy mode
		"Graphic Presets":         1,  // Low for performance
		"Shadow Quality":          0,
		"Texture Quality":         1,
		"Texture Anisotropy":      0,
		"Light Quality":           0,
		"Reflection Quality":      0,
		"Character Detail":        0,
		"Environment Detail":      0,
		"NVIDIA DLSS":             0,
		"AMD FSR":                 0,
		"Shader Model":            0,
		"Blend Mode":              0,
		"Perspective":             0,
		"VSync":                   0,  // Disabled for speed
		"Framerate Cap":           30, // Low since we're shopping, saves CPU
		"Resolution Scale":        100,
		"Window Mode":             2,  // Windowed

		// Audio - all muted for bot usage
		"Master Volume":           0,
		"Music Volume":            0,
		"Ambience Sound Volume":   0,
		"Combat Sound Volume":     0,
		"Item Sound Volume":       0,
		"Monster Sound Volume":    0,
		"NPC Sound Volume":        0,
		"Rain Sound Volume":       0,
		"UI Sound Volume":         0,
		"Cinematics Sound Volume": 0,

		// Gameplay
		"Always Run":              1,
		"Auto Gold Enabled":       1,
		"Item Name Display":       0,
		"Show HP Text":            1,
		"Chat Font Size":          1,
		"Item Tooltip Hotkey Appender": 1,

		// Screen resolution
		"Screen Width":            l.cfg.WindowWidth,
		"Screen Height":           l.cfg.WindowHeight,

		// Map and minimap
		"Automap Fade":            0,
		"Automap Center":          0,
		"Automap Party":           1,
		"Automap Names":           0,
	}
}

// startD2R launches the D2R executable.
func (l *Launcher) startD2R() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("D2R auto-start is only supported on Windows")
	}

	exePath := filepath.Join(l.cfg.D2RPath, "D2R.exe")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("D2R not found at %s", exePath)
	}

	args := []string{}
	// Add common launch arguments
	args = append(args, "-w") // Windowed mode

	l.logger.Info("Launching D2R",
		slog.String("path", exePath),
		slog.String("args", strings.Join(args, " ")),
	)

	cmd := exec.Command(exePath, args...)
	cmd.Dir = l.cfg.D2RPath
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start D2R: %w", err)
	}

	// Wait for D2R to initialize
	l.logger.Info("Waiting for D2R to initialize...")
	for i := 0; i < 60; i++ {
		time.Sleep(2 * time.Second)
		if l.isD2RRunning() {
			l.logger.Info("D2R is running")
			return nil
		}
	}

	return fmt.Errorf("D2R did not start within 120 seconds")
}

// isD2RRunning checks if a D2R process exists.
func (l *Launcher) isD2RRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}

	// Use tasklist to check for D2R.exe
	cmd := exec.Command("tasklist", "/fi", "imagename eq D2R.exe", "/fo", "csv", "/nh")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	return strings.Contains(string(output), "D2R.exe")
}

// ConfigureForLegacyMode sets up the configuration for legacy mode operation.
func (l *Launcher) ConfigureForLegacyMode() {
	l.logger.Info("Configuring for legacy mode")
	l.logger.Info("Legacy mode will be activated in-game via hotkey press")
	l.logger.Info("The bot reads game memory which works in both graphics modes")
	l.logger.Info("Legacy mode provides more consistent UI coordinates for the bot")
}

// PrintLaunchDiagnostics logs useful info about the D2R installation.
func (l *Launcher) PrintLaunchDiagnostics() {
	l.logger.Info("=== D2R Launch Diagnostics ===")

	// Check D2R installation
	exePath := filepath.Join(l.cfg.D2RPath, "D2R.exe")
	if _, err := os.Stat(exePath); err == nil {
		l.logger.Info("D2R found", slog.String("path", exePath))
	} else {
		l.logger.Warn("D2R NOT found", slog.String("path", exePath))
	}

	// Check settings file
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" {
		settingsPath := filepath.Join(userProfile, "Saved Games", "Diablo II Resurrected", "Settings.json")
		if _, err := os.Stat(settingsPath); err == nil {
			l.logger.Info("Settings.json found", slog.String("path", settingsPath))
		} else {
			l.logger.Info("Settings.json not found (will be created)", slog.String("path", settingsPath))
		}
	}

	l.logger.Info("Auto-start D2R: " + fmt.Sprintf("%v", l.cfg.AutoStartD2R))
	l.logger.Info("Auto-configure D2R: " + fmt.Sprintf("%v", l.cfg.AutoConfigureD2R))
	l.logger.Info("Auto-legacy mode: " + fmt.Sprintf("%v", l.cfg.AutoLegacyMode))
	l.logger.Info("Window size: " + fmt.Sprintf("%dx%d", l.cfg.WindowWidth, l.cfg.WindowHeight))
	l.logger.Info("==============================")
}
