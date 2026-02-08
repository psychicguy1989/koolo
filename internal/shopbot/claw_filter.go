package shopbot

import (
	"fmt"
	"strings"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/stat"
)

// ClawEvalResult represents the evaluation of a single claw against filter profiles.
type ClawEvalResult struct {
	Item           data.Item
	Matched        bool
	MatchedProfile string
	MatchReason    string
	SkipReason     string
	SkillsFound    map[string]int
	TotalSkillPts  int
}

// ClawFilter evaluates items against the configured claw filter profiles.
type ClawFilter struct {
	profiles []ClawFilterProfile
	logger   *ShopLogger
}

// NewClawFilter creates a filter from enabled profiles.
func NewClawFilter(profiles []ClawFilterProfile, logger *ShopLogger) *ClawFilter {
	var enabled []ClawFilterProfile
	for _, p := range profiles {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	return &ClawFilter{
		profiles: enabled,
		logger:   logger,
	}
}

// clawBaseTypes is the definitive list of assassin claw item types.
var clawBaseTypes = map[string]bool{
	// Normal
	"Katar":          true,
	"WristBlade":     true,
	"HatchetHands":   true,
	"Cestus":         true,
	"Claws":          true,
	"BladeTalons":    true,
	"ScissorsKatar":  true,
	// Exceptional
	"Quhab":          true,
	"WristSpike":     true,
	"Fascia":         true,
	"HandScythe":     true,
	"GreaterClaws":   true,
	"GreaterTalons":  true,
	"ScissorsQuhab":  true,
	// Elite
	"Suwayyah":       true,
	"WristSword":     true,
	"WarFist":        true,
	"BattleCestus":   true,
	"FeralClaws":     true,
	"RunicTalons":    true,
	"ScissorsSuwayyah": true,
}

// assassinSkillMap maps skill names to the stat IDs used in D2R memory.
// These are the +individual skill stats that appear on claws.
var assassinSkillMap = map[string]stat.ID{
	// Traps
	"FireBlast":            stat.SingleSkill,
	"ShockWeb":             stat.SingleSkill,
	"BladeSentinel":        stat.SingleSkill,
	"ChargedBoltSentry":    stat.SingleSkill,
	"WakeOfFire":           stat.SingleSkill,
	"BladeFury":            stat.SingleSkill,
	"LightningSentry":      stat.SingleSkill,
	"WakeOfInferno":        stat.SingleSkill,
	"DeathSentry":          stat.SingleSkill,
	"BladeShield":          stat.SingleSkill,
	// Shadow Disciplines
	"ClawMastery":          stat.SingleSkill,
	"PsychicHammer":        stat.SingleSkill,
	"BurstOfSpeed":         stat.SingleSkill,
	"WeaponBlock":          stat.SingleSkill,
	"CloakOfShadows":       stat.SingleSkill,
	"Fade":                 stat.SingleSkill,
	"ShadowWarrior":        stat.SingleSkill,
	"MindBlast":            stat.SingleSkill,
	"Venom":                stat.SingleSkill,
	"ShadowMaster":         stat.SingleSkill,
	// Martial Arts
	"TigerStrike":          stat.SingleSkill,
	"DragonTalon":          stat.SingleSkill,
	"FistsOfFire":          stat.SingleSkill,
	"DragonClaw":           stat.SingleSkill,
	"CobraStrike":          stat.SingleSkill,
	"ClawsOfThunder":       stat.SingleSkill,
	"BladesOfIce":          stat.SingleSkill,
	"DragonTail":           stat.SingleSkill,
	"DragonFlight":         stat.SingleSkill,
	"PhoenixStrike":        stat.SingleSkill,
}

// skillNameToID maps skill names to their D2R skill IDs for stat lookup.
// These are the actual skill IDs used in the SingleSkill stat layer.
var skillNameToID = map[string]int{
	// Traps tree
	"FireBlast":          251,
	"ShockWeb":           256,
	"BladeSentinel":      261,
	"ChargedBoltSentry":  252,
	"WakeOfFire":         257,
	"BladeFury":          262,
	"LightningSentry":    253,
	"WakeOfInferno":      258,
	"DeathSentry":        254,
	"BladeShield":        263,
	// Shadow Disciplines tree
	"ClawMastery":        245,
	"PsychicHammer":      250,
	"BurstOfSpeed":       258,
	"WeaponBlock":        260,
	"CloakOfShadows":     264,
	"Fade":               267,
	"ShadowWarrior":      268,
	"MindBlast":          273,
	"Venom":              278,
	"ShadowMaster":       279,
	// Martial Arts tree
	"TigerStrike":        246,
	"DragonTalon":        247,
	"FistsOfFire":        248,
	"DragonClaw":         249,
	"CobraStrike":        253,
	"ClawsOfThunder":     254,
	"BladesOfIce":        259,
	"DragonTail":         255,
	"DragonFlight":       260,
	"PhoenixStrike":      265,
}

// EvaluateItem checks if an item matches any enabled claw filter profile.
func (cf *ClawFilter) EvaluateItem(it data.Item) ClawEvalResult {
	result := ClawEvalResult{
		Item:        it,
		SkillsFound: make(map[string]int),
	}

	// Check if it's a claw type
	itemName := string(it.Name)
	if !clawBaseTypes[itemName] {
		result.SkipReason = fmt.Sprintf("not a claw type (%s)", itemName)
		return result
	}

	// Check quality
	qualityStr := qualityToString(it.Quality)

	// Extract all assassin skills from the item
	result.SkillsFound = extractAssassinSkills(it)
	for _, level := range result.SkillsFound {
		result.TotalSkillPts += level
	}

	// Evaluate against each enabled profile
	for _, profile := range cf.profiles {
		// Check claw type allowed in this profile
		if !isClawTypeAllowed(itemName, profile.ClawTypes) {
			continue
		}

		// Check quality allowed
		if it.Quality == item.QualityMagic && !profile.BuyMagic {
			continue
		}
		if it.Quality == item.QualityRare && !profile.BuyRare {
			continue
		}

		// Check required skills
		requiredMet := true
		for _, req := range profile.RequiredSkills {
			level, found := result.SkillsFound[req.SkillName]
			if !found || level < req.MinLevel {
				requiredMet = false
				break
			}
		}
		if !requiredMet {
			continue
		}

		// Check optional skills
		optionalMatched := 0
		for _, opt := range profile.OptionalSkills {
			if level, found := result.SkillsFound[opt.SkillName]; found && level >= opt.MinLevel {
				optionalMatched++
			}
		}
		if optionalMatched < profile.MinOptionalCount {
			continue
		}

		// Check total skill points
		if result.TotalSkillPts < profile.MinTotalSkills {
			continue
		}

		// Check additional stat requirements
		statsMet := true
		for _, sr := range profile.StatRequirements {
			if !checkStatRequirement(it, sr) {
				statsMet = false
				break
			}
		}
		if !statsMet {
			continue
		}

		// This profile matched
		result.Matched = true
		result.MatchedProfile = profile.Name
		result.MatchReason = fmt.Sprintf("Profile %q: %s [%s] with %s (total +%d skills)",
			profile.Name, itemName, qualityStr,
			formatSkills(result.SkillsFound), result.TotalSkillPts)
		break
	}

	if !result.Matched {
		result.SkipReason = fmt.Sprintf("%s [%s] with %s (total +%d) - no profile matched",
			itemName, qualityStr, formatSkills(result.SkillsFound), result.TotalSkillPts)
	}

	// Log the scan
	if cf.logger != nil {
		cf.logger.LogClawScan(itemName, qualityStr, result.SkillsFound, result.Matched, result.MatchReason)
		cf.logger.stats.ClawsScanned++
		if result.Matched {
			cf.logger.stats.ClawsMatched++
		}
	}

	return result
}

// extractAssassinSkills reads all +skill stats from an item.
func extractAssassinSkills(it data.Item) map[string]int {
	skills := make(map[string]int)

	for skillName, skillID := range skillNameToID {
		if s, found := it.FindStat(stat.SingleSkill, skillID); found && s.Value > 0 {
			skills[skillName] = s.Value
		}
	}

	// Also check for +all assassin skills (skill tab bonuses)
	if s, found := it.FindStat(stat.AddClassSkills, 2); found && s.Value > 0 { // 2 = assassin class
		skills["AllAssassinSkills"] = s.Value
	}

	// Check skill tab bonuses
	// Assassin tab IDs: 0=Traps, 1=ShadowDisciplines, 2=MartialArts (within class offset)
	if s, found := it.FindStat(stat.AddSkillTab, 48); found && s.Value > 0 { // Traps tab
		skills["TrapSkills"] = s.Value
	}
	if s, found := it.FindStat(stat.AddSkillTab, 49); found && s.Value > 0 { // Shadow tab
		skills["ShadowDisciplineSkills"] = s.Value
	}
	if s, found := it.FindStat(stat.AddSkillTab, 50); found && s.Value > 0 { // MA tab
		skills["MartialArtsSkills"] = s.Value
	}

	return skills
}

// checkStatRequirement evaluates a single stat filter against an item.
func checkStatRequirement(it data.Item, sr StatFilter) bool {
	statLower := strings.ToLower(sr.StatName)

	var statID stat.ID
	switch statLower {
	case "ias":
		statID = stat.IncreasedAttackSpeed
	case "fcr":
		statID = stat.FasterCastRate
	case "fhr":
		statID = stat.FasterHitRecovery
	case "frw":
		statID = stat.FasterRunWalk
	case "enhanceddamage", "ed":
		statID = stat.EnhancedDamage
	case "mindamage":
		statID = stat.MinDamage
	case "maxdamage":
		statID = stat.MaxDamage
	case "lifeleech":
		statID = stat.LifeSteal
	case "manaleech":
		statID = stat.ManaSteal
	case "sockets":
		statID = stat.NumSockets
	default:
		return false
	}

	if s, found := it.FindStat(statID, 0); found {
		return s.Value >= sr.MinValue
	}
	return false
}

func isClawTypeAllowed(name string, types []ClawType) bool {
	if len(types) == 0 {
		return true // No restrictions
	}
	for _, t := range types {
		if t.Enabled && t.Name == name {
			return true
		}
	}
	return false
}

func qualityToString(q item.Quality) string {
	switch q {
	case item.QualityNormal:
		return "Normal"
	case item.QualitySuperior:
		return "Superior"
	case item.QualityMagic:
		return "Magic"
	case item.QualityRare:
		return "Rare"
	case item.QualitySet:
		return "Set"
	case item.QualityUnique:
		return "Unique"
	default:
		return "Unknown"
	}
}
