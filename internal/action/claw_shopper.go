package action

import (
	"fmt"
	"log/slog"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/town"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
)

// RunClawShopper is the main entry point for the claw shopping feature.
// It shops Anya (Drehya) in Act 5 Harrogath for specific assassin claw base types,
// refreshing her inventory via the red portal to Nihlathak's Temple.
func RunClawShopper(cfg config.ClawShopperConfig) error {
	ctx := context.Get()

	if !cfg.Enabled {
		ctx.Logger.Debug("Claw Shopper disabled")
		return nil
	}

	clawBases := cfg.SelectedClawBases()
	if len(clawBases) == 0 {
		ctx.Logger.Warn("Claw Shopper: no claw types selected")
		return nil
	}

	ctx.Logger.Info("Starting Claw Shopper",
		slog.Int("maxAttempts", cfg.MaxAttempts),
		slog.Int("minGold", cfg.MinGoldReserve),
		slog.Any("clawBases", clawBases),
	)

	// Ensure we are in Harrogath (Act 5 town)
	if err := ensureInTown(area.Harrogath); err != nil {
		return fmt.Errorf("claw shopper: cannot reach Harrogath: %w", err)
	}
	ctx.RefreshGameData()

	// Ensure inventory space
	if !ensureTwoFreeColumnsStrict() {
		ctx.Logger.Warn("Claw Shopper: not enough inventory space")
		return nil
	}

	attempt := 0
	totalPurchased := 0

	for {
		attempt++
		// Check attempt limit (0 = unlimited)
		if cfg.MaxAttempts > 0 && attempt > cfg.MaxAttempts {
			ctx.Logger.Info("Claw Shopper: reached max attempts",
				slog.Int("maxAttempts", cfg.MaxAttempts),
				slog.Int("totalPurchased", totalPurchased),
			)
			break
		}

		// Check gold
		currentGold := ctx.Data.PlayerUnit.TotalPlayerGold()
		if currentGold < cfg.MinGoldReserve {
			ctx.Logger.Info("Claw Shopper: insufficient gold, stopping",
				slog.Int("gold", currentGold),
				slog.Int("minReserve", cfg.MinGoldReserve),
			)
			break
		}

		ctx.Logger.Info("Claw Shopper: attempt",
			slog.Int("attempt", attempt),
			slog.Int("gold", currentGold),
		)

		// Open Anya's trade menu (with 3-attempt retry)
		if err := clawShopperOpenAnyaTrade(ctx); err != nil {
			ctx.Logger.Error("Claw Shopper: failed to open Anya trade", slog.Any("err", err))
			break
		}

		// Scan all vendor tabs for matching claws and buy them
		purchased := clawShopperScanAndBuy(ctx, clawBases, cfg.MinGoldReserve)
		totalPurchased += purchased

		// Close menus
		step.CloseAllMenus()
		utils.Sleep(40)
		ctx.RefreshGameData()

		// If we bought something, stash it
		if purchased > 0 {
			ctx.Logger.Info("Claw Shopper: purchased claws, stashing",
				slog.Int("count", purchased),
			)
			if !ensureTwoFreeColumnsStrict() {
				ctx.Logger.Warn("Claw Shopper: inventory full after purchase, stopping")
				break
			}
		}

		// Check if we should continue (unlimited or still under limit)
		if cfg.MaxAttempts > 0 && attempt >= cfg.MaxAttempts {
			break
		}

		// Refresh Anya's shop via red portal
		if err := clawShopperRefreshViaRedPortal(ctx); err != nil {
			ctx.Logger.Warn("Claw Shopper: red portal refresh failed, trying waypoint",
				slog.Any("err", err),
			)
			// Fallback to waypoint refresh
			if err := refreshTownViaWaypoint(area.Harrogath); err != nil {
				ctx.Logger.Error("Claw Shopper: waypoint refresh also failed", slog.Any("err", err))
				break
			}
		}

		utils.Sleep(100)
		ctx.RefreshGameData()
	}

	ctx.Logger.Info("Claw Shopper finished",
		slog.Int("totalAttempts", attempt-1),
		slog.Int("totalPurchased", totalPurchased),
	)

	return nil
}

// clawShopperOpenAnyaTrade moves to Anya and opens her trade menu with up to 3 retries.
func clawShopperOpenAnyaTrade(ctx *context.Status) error {
	for retry := 0; retry < 3; retry++ {
		// Move to Anya
		if err := moveToVendor(npc.Drehya); err != nil {
			ctx.Logger.Debug("Claw Shopper: moveToVendor failed",
				slog.Int("retry", retry),
				slog.Any("err", err),
			)
			utils.Sleep(200)
			continue
		}
		utils.Sleep(60)
		ctx.RefreshGameData()

		// Interact with Anya
		if err := InteractNPC(npc.Drehya); err != nil {
			ctx.Logger.Debug("Claw Shopper: InteractNPC failed",
				slog.Int("retry", retry),
				slog.Any("err", err),
			)
			step.CloseAllMenus()
			utils.Sleep(200)
			continue
		}

		// Open trade (Anya: HOME -> DOWN -> ENTER)
		openVendorTrade(npc.Drehya)

		// Wait for shop to populate
		for i := 0; i < 10; i++ {
			ctx.RefreshGameData()
			if ctx.Data.OpenMenus.NPCShop {
				break
			}
			utils.Sleep(60)
		}

		if ctx.Data.OpenMenus.NPCShop {
			// Wait for vendor items to load
			for i := 0; i < 5; i++ {
				if len(ctx.Data.Inventory.ByLocation(item.LocationVendor)) > 0 {
					break
				}
				utils.Sleep(60)
				ctx.RefreshGameData()
			}
			return nil
		}

		ctx.Logger.Debug("Claw Shopper: trade menu didn't open",
			slog.Int("retry", retry),
		)
		step.CloseAllMenus()
		utils.Sleep(300)
	}

	return fmt.Errorf("failed to open Anya's trade menu after 3 attempts")
}

// clawShopperScanAndBuy scans all vendor tabs for claws matching the selected bases and buys them.
func clawShopperScanAndBuy(ctx *context.Status, clawBases []string, minGold int) int {
	purchased := 0

	for tab := 1; tab <= 4; tab++ {
		switchVendorTabFast(tab)
		ctx.RefreshGameData()

		vendorItems := ctx.Data.Inventory.ByLocation(item.LocationVendor)
		for _, it := range vendorItems {
			if (it.Location.Page + 1) != tab {
				continue
			}

			// Check if this item's Name matches any of our selected claw bases
			if !clawBaseMatch(it, clawBases) {
				continue
			}

			// Gold check before buying
			if ctx.Data.PlayerUnit.TotalPlayerGold() < minGold {
				ctx.Logger.Info("Claw Shopper: gold below reserve, stopping scan",
					slog.Int("gold", ctx.Data.PlayerUnit.TotalPlayerGold()),
				)
				return purchased
			}

			// Check inventory space
			if !hasTwoFreeColumns() {
				if !stashAndReturnToVendor(npc.Drehya, tab) {
					ctx.Logger.Warn("Claw Shopper: stash+return failed")
					return purchased
				}
				switchVendorTabFast(tab)
				ctx.RefreshGameData()
			}

			// Buy the claw
			coords := ui.GetScreenCoordsForItem(it)
			ctx.Logger.Info("Claw Shopper: buying claw",
				slog.String("name", string(it.Name)),
				slog.Int("tab", tab),
				slog.Int("x", coords.X),
				slog.Int("y", coords.Y),
			)

			town.BuyItem(it, 1)
			purchased++

			utils.Sleep(40)
			ctx.RefreshGameData()
		}
	}

	return purchased
}

// clawBaseMatch returns true if the item's Name matches any of the selected claw base names.
func clawBaseMatch(it data.Item, bases []string) bool {
	name := string(it.Name)
	for _, b := range bases {
		if name == b {
			return true
		}
	}
	return false
}

// clawShopperRefreshViaRedPortal refreshes Anya's inventory by entering and exiting the red portal.
func clawShopperRefreshViaRedPortal(ctx *context.Status) error {
	// Walk toward the red portal area near Anya
	_ = MoveToCoords(data.Position{X: 5116, Y: 5121})
	utils.Sleep(600)
	ctx.RefreshGameData()

	// Find the red portal
	redPortal, found := ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return fmt.Errorf("claw shopper: red portal not found in Harrogath")
	}

	// Enter the red portal
	if err := InteractObject(redPortal, func() bool {
		return ctx.Data.AreaData.Area == area.NihlathaksTemple &&
			ctx.Data.AreaData.IsInside(ctx.Data.PlayerUnit.Position)
	}); err != nil {
		return fmt.Errorf("claw shopper: failed to enter red portal: %w", err)
	}

	utils.Sleep(120)
	ctx.RefreshGameData()

	// Return to town via the red portal in the temple
	if err := returnToTownViaAnyaRedPortalFromTemple(); err != nil {
		// Fallback: use TP
		ctx.Logger.Debug("Claw Shopper: temple red portal return failed, using TP")
		if err := ReturnTown(); err != nil {
			return fmt.Errorf("claw shopper: failed to return to town: %w", err)
		}
	}

	utils.Sleep(80)
	ctx.RefreshGameData()
	return nil
}
