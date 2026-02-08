// cmd/shopbot/main.go
//
// D2R Shop Bot - Standalone Claw Shopping Bot for Diablo 2 Resurrected
//
// This is a standalone program that ONLY does shopping at Anya (Drehya) in
// Act 5 Harrogath for assassin claws with specific skills and stats.
//
// Features:
//   - Auto-launches D2R and configures legacy mode + optimal settings
//   - Paths from any town to Harrogath via waypoint
//   - Walks to Anya and opens her shop
//   - Scans claws against configurable filter profiles
//   - Refreshes vendor inventory via red portal or waypoint hop
//   - Visual overlay with markers for waypoints, NPCs, and path
//   - Comprehensive logging with QoL diagnostics
//
// Usage:
//
//	shopbot                       # Run with default config (shopbot.yaml)
//	shopbot -config my_config.yaml # Run with custom config
//	shopbot -generate-config      # Generate a default config file
//	shopbot -list-profiles        # Show available claw filter profiles
//	shopbot -validate             # Validate config without running
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
	defer logger.PrintDiagnosticSummary()

	printBanner()

	// Validate config
	if errs := cfg.Validate(); len(errs) > 0 {
		logger.Warn("Configuration warnings:")
		for _, e := range errs {
			logger.Warn("  - " + e)
		}
	}

	// Initialize and run launcher
	launcher := shopbot.NewLauncher(cfg, logger)
	launcher.PrintLaunchDiagnostics()

	if cfg.AutoStartD2R {
		if err := launcher.EnsureD2RRunning(); err != nil {
			logger.LogError("launcher", fmt.Sprintf("Failed to start D2R: %v", err))
			logger.Info("Please start D2R manually and re-run the shop bot")
			os.Exit(1)
		}
	}

	if cfg.AutoLegacyMode {
		launcher.ConfigureForLegacyMode()
	}

	// Create and start engine
	engine := shopbot.NewEngine(cfg, logger)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		engine.Stop()
	}()

	// NOTE: The shop bot engine requires the Koolo supervisor infrastructure
	// to be running (memory reader, HID, pathfinder, etc.).
	// When run standalone, it integrates with a running Koolo instance's context.
	//
	// To use this as a fully standalone program, start Koolo first,
	// then run this as a separate shopping-focused bot that uses the
	// existing game connection.
	logger.Info("Shop Bot engine initialized")
	logger.Info("Waiting for game context from Koolo supervisor...")
	logger.Info("Ensure Koolo is running and a character is in-game")

	if err := engine.Run(); err != nil {
		logger.LogError("engine", fmt.Sprintf("Shop bot error: %v", err))
		os.Exit(1)
	}

	logger.Info("Shop bot finished successfully")
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
	fmt.Println("  - d2r_path: Path to your D2R installation")
	fmt.Println("  - character_name: Your character name")
	fmt.Println("  - profiles: Claw filter profiles (skills, stats)")
	fmt.Println("\nThen run: shopbot -config " + *configPath)
}

func doListProfiles() {
	// Try loading from config first
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

		// Show claw types
		var types []string
		for _, ct := range p.ClawTypes {
			if ct.Enabled {
				types = append(types, ct.Name)
			}
		}
		fmt.Printf("    Claw types: %s\n", strings.Join(types, ", "))

		// Show required skills
		if len(p.RequiredSkills) > 0 {
			fmt.Printf("    Required skills:\n")
			for _, s := range p.RequiredSkills {
				fmt.Printf("      - +%d %s (minimum)\n", s.MinLevel, s.SkillName)
			}
		}

		// Show optional skills
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
  ==========================================================
`
	fmt.Println(banner)
}

func printUsage() {
	fmt.Println("D2R Shop Bot - Standalone Claw Shopping Bot")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  shopbot                          Run with default config (shopbot.yaml)")
	fmt.Println("  shopbot -config path.yaml        Run with custom config file")
	fmt.Println("  shopbot -generate-config         Generate a default config file")
	fmt.Println("  shopbot -list-profiles           Show claw filter profiles")
	fmt.Println("  shopbot -validate                Validate config without running")
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println("  1. Run 'shopbot -generate-config' to create shopbot.yaml")
	fmt.Println("  2. Edit shopbot.yaml with your D2R path and character name")
	fmt.Println("  3. Configure claw filter profiles for the skills you want")
	fmt.Println("  4. Start Koolo, then run 'shopbot' to begin shopping")
	fmt.Println()
	fmt.Println("The bot will:")
	fmt.Println("  - Auto-configure D2R settings for optimal bot operation")
	fmt.Println("  - Switch to legacy graphics mode if configured")
	fmt.Println("  - Navigate from any town to Harrogath via waypoint")
	fmt.Println("  - Walk to Anya and open her shop")
	fmt.Println("  - Scan claws against your filter profiles")
	fmt.Println("  - Buy matching claws automatically")
	fmt.Println("  - Refresh vendors and repeat")
	fmt.Println("  - Show real-time overlay at http://localhost:8099")
	fmt.Println()
	fmt.Println("Available Assassin Skills for Filters:")
	fmt.Println("  Traps: FireBlast, ShockWeb, BladeSentinel, ChargedBoltSentry,")
	fmt.Println("         WakeOfFire, BladeFury, LightningSentry, WakeOfInferno,")
	fmt.Println("         DeathSentry, BladeShield")
	fmt.Println("  Shadow: ClawMastery, PsychicHammer, BurstOfSpeed, WeaponBlock,")
	fmt.Println("          CloakOfShadows, Fade, ShadowWarrior, MindBlast,")
	fmt.Println("          Venom, ShadowMaster")
	fmt.Println("  Martial: TigerStrike, DragonTalon, FistsOfFire, DragonClaw,")
	fmt.Println("           CobraStrike, ClawsOfThunder, BladesOfIce, DragonTail,")
	fmt.Println("           DragonFlight, PhoenixStrike")
	fmt.Println()
	fmt.Println("Additional Stats for Filters:")
	fmt.Println("  ias, fcr, fhr, frw, enhanceddamage, mindamage, maxdamage,")
	fmt.Println("  lifeleech, manaleech, sockets")
}
