package run

import (
	"log/slog"

	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
)

type ClawShopper struct{}

func NewClawShopper() *ClawShopper { return &ClawShopper{} }

func (r ClawShopper) Name() string { return string(config.ClawShopperRun) }

func (r ClawShopper) CheckConditions(parameters *RunParameters) SequencerResult {
	ctx := context.Get()
	cfg := ctx.Data.CharacterCfg.ClawShopper
	if !cfg.Enabled || len(cfg.SelectedClawBases()) == 0 {
		return SequencerSkip
	}
	return SequencerOk
}

func (r ClawShopper) Run(parameters *RunParameters) error {
	ctx := context.Get()
	cfg := ctx.Data.CharacterCfg.ClawShopper

	ctx.Logger.Info("Starting Claw Shopper run",
		slog.Int("maxAttempts", cfg.MaxAttempts),
		slog.Int("minGold", cfg.MinGoldReserve),
		slog.Any("claws", cfg.SelectedClawBases()),
	)

	return action.RunClawShopper(cfg)
}

// SkipTownRoutines returns true so the claw shopper handles its own town navigation.
func (r ClawShopper) SkipTownRoutines() bool {
	return true
}
