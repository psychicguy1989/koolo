package shopbot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ClawType represents a claw base type that can be shopped.
type ClawType struct {
	Name    string `yaml:"name"`
	Enabled bool   `yaml:"enabled"`
}

// SkillFilter defines how a specific skill should be evaluated on claws.
type SkillFilter struct {
	SkillName string `yaml:"skill_name"`
	MinLevel  int    `yaml:"min_level"` // Minimum +skill level required (e.g., 3 means +3 or higher)
}

// StatFilter defines a numeric stat requirement.
type StatFilter struct {
	StatName string `yaml:"stat_name"` // e.g. "ias", "fcr", "fhr", "enhanceddamage"
	MinValue int    `yaml:"min_value"`
}

// ClawFilterProfile is a named filter configuration for claws.
type ClawFilterProfile struct {
	Name             string        `yaml:"name"`
	Enabled          bool          `yaml:"enabled"`
	Description      string        `yaml:"description"`
	ClawTypes        []ClawType    `yaml:"claw_types"`         // Which base claw types to look for
	RequiredSkills   []SkillFilter `yaml:"required_skills"`    // ALL of these must be present
	OptionalSkills   []SkillFilter `yaml:"optional_skills"`    // At least MinOptionalCount of these
	MinOptionalCount int           `yaml:"min_optional_count"` // How many optional skills must match
	MinTotalSkills   int           `yaml:"min_total_skills"`   // Minimum sum of +skill levels across all skills
	StatRequirements []StatFilter  `yaml:"stat_requirements"`  // Additional stat filters (IAS, etc.)
	MaxPrice         int           `yaml:"max_price"`          // Max gold to spend per claw (0 = unlimited)
	BuyMagic         bool          `yaml:"buy_magic"`          // Buy magic quality claws
	BuyRare          bool          `yaml:"buy_rare"`           // Buy rare quality claws
}

// ShopBotConfig is the full configuration for the standalone shop bot.
type ShopBotConfig struct {
	// D2R Settings
	D2RPath          string `yaml:"d2r_path"`            // Path to D2R installation
	AutoStartD2R     bool   `yaml:"auto_start_d2r"`      // Automatically launch D2R
	AutoLegacyMode   bool   `yaml:"auto_legacy_mode"`    // Auto-switch to legacy graphics
	AutoConfigureD2R bool   `yaml:"auto_configure_d2r"`  // Auto-apply optimal settings
	WindowWidth      int    `yaml:"window_width"`
	WindowHeight     int    `yaml:"window_height"`

	// Account
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Realm    string `yaml:"realm"` // "us.actual.battle.net", "eu.actual.battle.net", etc.
	AuthMode string `yaml:"auth_mode"` // "token", "password", "none"

	// Character
	CharacterName string `yaml:"character_name"`
	Difficulty    string `yaml:"difficulty"` // "normal", "nightmare", "hell"
	GameNameBase  string `yaml:"game_name_base"`
	GamePassword  string `yaml:"game_password"`

	// Shopping behavior
	RefreshesPerSession int  `yaml:"refreshes_per_session"` // How many shop refreshes per game
	MinGoldReserve      int  `yaml:"min_gold_reserve"`      // Don't spend below this
	AutoCreateGame      bool `yaml:"auto_create_game"`      // Automatically create new games
	GamesPerSession     int  `yaml:"games_per_session"`     // Max games before stopping (0 = unlimited)
	DelayBetweenGames   int  `yaml:"delay_between_games"`   // Seconds between games

	// Pathing
	StartAct       int  `yaml:"start_act"`        // Which act to start from (1-5, 0=auto-detect)
	UseWaypoint    bool `yaml:"use_waypoint"`      // Use waypoint to get to Harrogath
	PathingTimeout int  `yaml:"pathing_timeout"`   // Seconds before pathing gives up

	// Visual Overlay
	Overlay OverlayConfig `yaml:"overlay"`

	// Logging
	Logging LoggingConfig `yaml:"logging"`

	// Claw filters
	Profiles []ClawFilterProfile `yaml:"profiles"`
}

// OverlayConfig controls the debug/visual overlay system.
type OverlayConfig struct {
	Enabled          bool `yaml:"enabled"`
	ShowWaypoints    bool `yaml:"show_waypoints"`
	ShowNPCs         bool `yaml:"show_npcs"`
	ShowPlayerPos    bool `yaml:"show_player_pos"`
	ShowPathLines    bool `yaml:"show_path_lines"`
	ShowAnyaMarker   bool `yaml:"show_anya_marker"`
	ShowItemHighlight bool `yaml:"show_item_highlight"`
	MarkerSize       int  `yaml:"marker_size"`
	RefreshRateMs    int  `yaml:"refresh_rate_ms"`
	OverlayPort      int  `yaml:"overlay_port"` // Web overlay on this port
}

// LoggingConfig controls logging output.
type LoggingConfig struct {
	Level       string `yaml:"level"`         // "debug", "info", "warn", "error"
	LogToFile   bool   `yaml:"log_to_file"`
	LogDir      string `yaml:"log_dir"`
	MaxLogSizeMB int   `yaml:"max_log_size_mb"`
	Verbose     bool   `yaml:"verbose"`       // Extra diagnostic output
	LogPathing  bool   `yaml:"log_pathing"`   // Log pathfinding decisions
	LogShopping bool   `yaml:"log_shopping"`  // Log every item scan result
	LogGameState bool  `yaml:"log_game_state"` // Log area/position changes
}

// DefaultConfig returns a fully populated default configuration.
func DefaultConfig() ShopBotConfig {
	return ShopBotConfig{
		D2RPath:          `C:\Program Files (x86)\Diablo II Resurrected`,
		AutoStartD2R:     false,
		AutoLegacyMode:   true,
		AutoConfigureD2R: true,
		WindowWidth:      1280,
		WindowHeight:     720,

		AuthMode:    "token",
		Realm:       "us.actual.battle.net",
		Difficulty:  "hell",
		GameNameBase: "shop",
		GamePassword: "",

		RefreshesPerSession: 100,
		MinGoldReserve:      50000,
		AutoCreateGame:      true,
		GamesPerSession:     0,
		DelayBetweenGames:   5,

		StartAct:       0,
		UseWaypoint:    true,
		PathingTimeout: 30,

		Overlay: OverlayConfig{
			Enabled:          true,
			ShowWaypoints:    true,
			ShowNPCs:         true,
			ShowPlayerPos:    true,
			ShowPathLines:    true,
			ShowAnyaMarker:   true,
			ShowItemHighlight: true,
			MarkerSize:       8,
			RefreshRateMs:    100,
			OverlayPort:      8099,
		},

		Logging: LoggingConfig{
			Level:        "info",
			LogToFile:    true,
			LogDir:       "logs/shopbot",
			MaxLogSizeMB: 50,
			Verbose:      false,
			LogPathing:   true,
			LogShopping:  true,
			LogGameState: true,
		},

		Profiles: DefaultClawProfiles(),
	}
}

// DefaultClawProfiles returns a set of predefined claw shopping profiles.
func DefaultClawProfiles() []ClawFilterProfile {
	return []ClawFilterProfile{
		{
			Name:        "Godly Trap Claws",
			Enabled:     true,
			Description: "Look for +3 Traps claws with Lightning Sentry and Death Sentry",
			ClawTypes: []ClawType{
				{Name: "GreaterTalons", Enabled: true},
				{Name: "RunicTalons", Enabled: true},
				{Name: "FeralClaws", Enabled: true},
				{Name: "ScissorsQuhab", Enabled: true},
			},
			RequiredSkills: []SkillFilter{
				{SkillName: "LightningSentry", MinLevel: 3},
			},
			OptionalSkills: []SkillFilter{
				{SkillName: "DeathSentry", MinLevel: 3},
				{SkillName: "FireBlast", MinLevel: 3},
				{SkillName: "ShockWeb", MinLevel: 3},
				{SkillName: "ChargedBoltSentry", MinLevel: 3},
				{SkillName: "WakeOfFire", MinLevel: 3},
				{SkillName: "WakeOfInferno", MinLevel: 3},
			},
			MinOptionalCount: 1,
			MinTotalSkills:   6,
			MaxPrice:         0,
			BuyMagic:         true,
			BuyRare:          false,
		},
		{
			Name:        "Good Trap Claws",
			Enabled:     true,
			Description: "Buy any claws with +2 or more to Lightning Sentry",
			ClawTypes: []ClawType{
				{Name: "GreaterTalons", Enabled: true},
				{Name: "RunicTalons", Enabled: true},
				{Name: "FeralClaws", Enabled: true},
				{Name: "ScissorsQuhab", Enabled: true},
				{Name: "WristSword", Enabled: true},
				{Name: "Claws", Enabled: true},
			},
			RequiredSkills: []SkillFilter{
				{SkillName: "LightningSentry", MinLevel: 2},
			},
			OptionalSkills: []SkillFilter{
				{SkillName: "DeathSentry", MinLevel: 1},
				{SkillName: "WakeOfFire", MinLevel: 1},
				{SkillName: "MindBlast", MinLevel: 1},
			},
			MinOptionalCount: 0,
			MinTotalSkills:   3,
			MaxPrice:         0,
			BuyMagic:         true,
			BuyRare:          true,
		},
		{
			Name:        "Martial Arts Claws",
			Enabled:     false,
			Description: "Shopping for martial arts assassin claws",
			ClawTypes: []ClawType{
				{Name: "GreaterTalons", Enabled: true},
				{Name: "RunicTalons", Enabled: true},
				{Name: "FeralClaws", Enabled: true},
			},
			RequiredSkills: []SkillFilter{
				{SkillName: "PhoenixStrike", MinLevel: 2},
			},
			OptionalSkills: []SkillFilter{
				{SkillName: "TigerStrike", MinLevel: 2},
				{SkillName: "DragonTalon", MinLevel: 2},
				{SkillName: "DragonClaw", MinLevel: 2},
				{SkillName: "CobraStrike", MinLevel: 2},
				{SkillName: "ClawsOfThunder", MinLevel: 2},
				{SkillName: "BladesOfIce", MinLevel: 2},
				{SkillName: "Venom", MinLevel: 1},
			},
			MinOptionalCount: 1,
			MinTotalSkills:   5,
			MaxPrice:         0,
			BuyMagic:         true,
			BuyRare:          true,
		},
		{
			Name:        "Shadow Discipline Claws",
			Enabled:     false,
			Description: "Claws with Burst of Speed, Fade, Venom, Mind Blast",
			ClawTypes: []ClawType{
				{Name: "GreaterTalons", Enabled: true},
				{Name: "RunicTalons", Enabled: true},
				{Name: "FeralClaws", Enabled: true},
			},
			RequiredSkills: []SkillFilter{},
			OptionalSkills: []SkillFilter{
				{SkillName: "BurstOfSpeed", MinLevel: 2},
				{SkillName: "Fade", MinLevel: 2},
				{SkillName: "Venom", MinLevel: 2},
				{SkillName: "MindBlast", MinLevel: 2},
				{SkillName: "ShadowMaster", MinLevel: 2},
				{SkillName: "WeaponBlock", MinLevel: 2},
			},
			MinOptionalCount: 2,
			MinTotalSkills:   5,
			MaxPrice:         0,
			BuyMagic:         true,
			BuyRare:          true,
		},
	}
}

// LoadConfig loads configuration from a YAML file.
func LoadConfig(path string) (*ShopBotConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	return &cfg, nil
}

// SaveConfig writes configuration to a YAML file.
func SaveConfig(cfg *ShopBotConfig, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Validate checks the config for obvious errors.
func (c *ShopBotConfig) Validate() []string {
	var errs []string

	if c.D2RPath == "" {
		errs = append(errs, "d2r_path is required")
	}
	if c.CharacterName == "" {
		errs = append(errs, "character_name is required")
	}

	enabledProfiles := 0
	for _, p := range c.Profiles {
		if p.Enabled {
			enabledProfiles++
			enabledClaws := 0
			for _, ct := range p.ClawTypes {
				if ct.Enabled {
					enabledClaws++
				}
			}
			if enabledClaws == 0 {
				errs = append(errs, fmt.Sprintf("profile %q has no claw types enabled", p.Name))
			}
		}
	}
	if enabledProfiles == 0 {
		errs = append(errs, "no claw filter profiles are enabled")
	}

	validDifficulties := map[string]bool{"normal": true, "nightmare": true, "hell": true}
	if !validDifficulties[strings.ToLower(c.Difficulty)] {
		errs = append(errs, fmt.Sprintf("invalid difficulty %q (must be normal/nightmare/hell)", c.Difficulty))
	}

	return errs
}

// EnabledProfiles returns only the enabled profiles.
func (c *ShopBotConfig) EnabledProfiles() []ClawFilterProfile {
	var result []ClawFilterProfile
	for _, p := range c.Profiles {
		if p.Enabled {
			result = append(result, p)
		}
	}
	return result
}
