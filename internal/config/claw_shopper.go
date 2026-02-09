package config

// ClawShopperConfig holds settings for the assassin claw shopping feature.
// It shops Anya (Drehya) in Act 5 for specific claw base types and refreshes
// her inventory via the red portal to Nihlathak's Temple.
type ClawShopperConfig struct {
	Enabled        bool `yaml:"enabled"`
	MaxAttempts    int  `yaml:"max_attempts"`    // 0 = unlimited
	MinGoldReserve int  `yaml:"min_gold_reserve"` // stop shopping below this gold

	// Individual claw base selection
	RunicTalons   bool `yaml:"runic_talons"`
	GreaterTalons bool `yaml:"greater_talons"`
	FeralClaws    bool `yaml:"feral_claws"`
	Suwayyah      bool `yaml:"suwayyah"`
}

// SelectedClawBases returns the internal item names for enabled claw types.
// These match the d2go item.Name values (PascalCase).
func (c ClawShopperConfig) SelectedClawBases() []string {
	out := make([]string, 0, 4)
	if c.RunicTalons {
		out = append(out, "RunicTalons")
	}
	if c.GreaterTalons {
		out = append(out, "GreaterTalons")
	}
	if c.FeralClaws {
		out = append(out, "FeralClaws")
	}
	if c.Suwayyah {
		out = append(out, "Suwayyah")
	}
	return out
}
