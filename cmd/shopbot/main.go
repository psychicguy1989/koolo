// cmd/shopbot/main.go
//
// D2R Shop Bot - Standalone Claw Shopping Bot for Diablo 2 Resurrected
//
// This is a fully standalone program that does NOT require Koolo to be running.
// It detects running D2R processes, attaches to them directly, and provides
// a web GUI for controlling the shopping bot.
//
// Features:
//   - Detects running D2R clients and shows them in a dropdown
//   - Step-by-step diagnostic tests before shopping
//   - Walks to Anya and scans her shop for matching claws
//   - Configurable claw filter profiles (skills, stats)
//   - In-game settings configurator via ESC menu
//   - Auto legacy mode switching
//   - Random break system
//   - Web overlay with markers and live stats
//   - Comprehensive logging in the GUI
//
// Usage:
//
//	shopbot                          Run with default config (shopbot.yaml)
//	shopbot -config my_config.yaml   Run with custom config file
//	shopbot -generate-config         Generate a default config file
//	shopbot -list-profiles           Show available claw filter profiles
//	shopbot -validate                Validate config without running
//	shopbot -port 8099               Set GUI server port
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hectorgimenez/koolo/internal/shopbot"
)

var (
	configPath     = flag.String("config", "shopbot.yaml", "Path to shop bot config file")
	generateConfig = flag.Bool("generate-config", false, "Generate a default shopbot.yaml config file")
	listProfiles   = flag.Bool("list-profiles", false, "List available claw filter profiles")
	validateOnly   = flag.Bool("validate", false, "Validate config without running")
	port           = flag.Int("port", 0, "GUI server port (overrides config)")
	showHelp       = flag.Bool("h", false, "Show help")
)

func main() {
	flag.Parse()

	if *showHelp {
		printUsage()
		return
	}

	if *generateConfig {
		doGenerateConfig()
		return
	}

	if *listProfiles {
		doListProfiles()
		return
	}

	// Load config
	cfg, err := loadOrCreateConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if *validateOnly {
		doValidate(cfg)
		return
	}

	// Initialize logger
	logger, err := shopbot.NewShopLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	printBanner()

	// Validate config
	if errs := cfg.Validate(); len(errs) > 0 {
		logger.Warn("Configuration warnings:")
		for _, e := range errs {
			logger.Warn("  - " + e)
		}
	}

	// Override port if specified via flag
	guiPort := cfg.Overlay.OverlayPort
	if *port > 0 {
		guiPort = *port
	}
	if guiPort == 0 {
		guiPort = 8099
	}

	// Create and start GUI server (standalone - no Koolo needed)
	gui := shopbot.NewGUIServer(cfg, logger, guiPort)
	if err := gui.Start(); err != nil {
		logger.LogError("gui", fmt.Sprintf("Failed to start GUI server: %v", err))
		os.Exit(1)
	}

	fmt.Println("=========================================")
	fmt.Printf("  D2R Shop Bot GUI running at:\n")
	fmt.Printf("  http://localhost:%d\n", guiPort)
	fmt.Println("=========================================")
	fmt.Println()
	fmt.Println("Open the URL above in your browser to:")
	fmt.Println("  1. Detect and attach to a D2R process")
	fmt.Println("  2. Run diagnostic tests")
	fmt.Println("  3. Configure in-game settings")
	fmt.Println("  4. Start/stop shopping")
	fmt.Println()
	fmt.Println("Press Ctrl+C to stop the bot.")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
	logger.Info("Shutdown signal received")
}

func loadOrCreateConfig(path string) (*shopbot.ShopBotConfig, error) {
	cfg, err := shopbot.LoadConfig(path)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("Config file %s not found. Creating default config...\n", path)
			defaultCfg := shopbot.DefaultConfig()
			if err := shopbot.SaveConfig(&defaultCfg, path); err != nil {
				return nil, fmt.Errorf("creating default config: %w", err)
			}
			fmt.Printf("Default config created at %s\n", path)
			fmt.Println("Please edit the config file and run again.")
			os.Exit(0)
		}
		return nil, err
	}
	return cfg, nil
}

func doGenerateConfig() {
	cfg := shopbot.DefaultConfig()
	if err := shopbot.SaveConfig(&cfg, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Default config generated: %s\n", *configPath)
	fmt.Println("\nEdit the config file to set your preferences:")
	fmt.Println("  - character_name: Your character name")
	fmt.Println("  - profiles: Claw filter profiles (skills, stats)")
	fmt.Println("\nThen run: shopbot")
}

func doListProfiles() {
	cfg, err := shopbot.LoadConfig(*configPath)
	if err != nil {
		cfg = &shopbot.ShopBotConfig{Profiles: shopbot.DefaultClawProfiles()}
	}

	fmt.Println("=== Claw Filter Profiles ===")
	fmt.Println()

	for i, p := range cfg.Profiles {
		status := "DISABLED"
		if p.Enabled {
			status = "ENABLED"
		}
		fmt.Printf("[%d] %s [%s]\n", i+1, p.Name, status)
		fmt.Printf("    %s\n", p.Description)

		var types []string
		for _, ct := range p.ClawTypes {
			if ct.Enabled {
				types = append(types, ct.Name)
			}
		}
		fmt.Printf("    Claw types: %s\n", strings.Join(types, ", "))

		if len(p.RequiredSkills) > 0 {
			fmt.Printf("    Required skills:\n")
			for _, s := range p.RequiredSkills {
				fmt.Printf("      - +%d %s (minimum)\n", s.MinLevel, s.SkillName)
			}
		}

		if len(p.OptionalSkills) > 0 {
			fmt.Printf("    Optional skills (need %d):\n", p.MinOptionalCount)
			for _, s := range p.OptionalSkills {
				fmt.Printf("      - +%d %s\n", s.MinLevel, s.SkillName)
			}
		}

		fmt.Printf("    Min total +skills: %d\n", p.MinTotalSkills)
		fmt.Printf("    Buy magic: %v, Buy rare: %v\n", p.BuyMagic, p.BuyRare)
		fmt.Println()
	}
}

func doValidate(cfg *shopbot.ShopBotConfig) {
	errs := cfg.Validate()
	if len(errs) == 0 {
		fmt.Println("Config is valid!")
		fmt.Printf("  Character: %s\n", cfg.CharacterName)
		fmt.Printf("  Difficulty: %s\n", cfg.Difficulty)
		fmt.Printf("  Refreshes per session: %d\n", cfg.RefreshesPerSession)
		fmt.Printf("  Active profiles: %d\n", len(cfg.EnabledProfiles()))
		for _, p := range cfg.EnabledProfiles() {
			fmt.Printf("    - %s: %s\n", p.Name, p.Description)
		}
	} else {
		fmt.Println("Config has errors:")
		for _, e := range errs {
			fmt.Printf("  - %s\n", e)
		}
		os.Exit(1)
	}
}

func printBanner() {
	banner := `
 ____  ____  ____    ____  _   _  ___  ____    ____   ___  _____
|  _ \|___ \|  _ \  / ___|| | | |/ _ \|  _ \  | __ ) / _ \|_   _|
| | | | __) | |_) | \___ \| |_| | | | | |_) | |  _ \| | | | | |
| |_| |/ __/|  _ <   ___) |  _  | |_| |  __/  | |_) | |_| | | |
|____/|_____|_| \_\ |____/|_| |_|\___/|_|     |____/ \___/  |_|

  Standalone Anya Claw Shopping Bot for Diablo 2 Resurrected
  ======================== v2.0 ============================
  No Koolo required - fully standalone operation
`
	fmt.Println(banner)
}

func printUsage() {
	fmt.Println("D2R Shop Bot - Standalone Claw Shopping Bot")
	fmt.Println()
	fmt.Println("This bot runs FULLY STANDALONE - no Koolo needed!")
	fmt.Println("It provides a web GUI to control everything from your browser.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  shopbot                          Run with default config (shopbot.yaml)")
	fmt.Println("  shopbot -config path.yaml        Run with custom config file")
	fmt.Println("  shopbot -generate-config         Generate a default config file")
	fmt.Println("  shopbot -list-profiles           Show claw filter profiles")
	fmt.Println("  shopbot -validate                Validate config without running")
	fmt.Println("  shopbot -port 8099               Set GUI server port")
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println("  1. Run 'shopbot -generate-config' to create shopbot.yaml")
	fmt.Println("  2. Edit shopbot.yaml to configure claw filter profiles")
	fmt.Println("  3. Start D2R and join a game with your assassin in Harrogath")
	fmt.Println("  4. Run 'shopbot' and open http://localhost:8099 in your browser")
	fmt.Println("  5. Click 'Detect D2R' to find your game, then 'Attach'")
	fmt.Println("  6. Run diagnostic tests, then start shopping!")
	fmt.Println()
	fmt.Println("The bot will:")
	fmt.Println("  - Detect running D2R.exe processes automatically")
	fmt.Println("  - Attach to the selected D2R window (multi-client support)")
	fmt.Println("  - Run step-by-step tests to verify everything works")
	fmt.Println("  - Configure in-game settings and switch to legacy mode")
	fmt.Println("  - Navigate from any town to Harrogath via waypoint")
	fmt.Println("  - Walk to Anya and scan her shop for matching claws")
	fmt.Println("  - Buy matching claws automatically")
	fmt.Println("  - Refresh vendors and repeat")
	fmt.Println("  - Take random breaks if configured")
}
