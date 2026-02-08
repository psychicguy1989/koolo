package shopbot

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/area"
	"github.com/hectorgimenez/d2go/pkg/data/item"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/koolo/internal/action"
	"github.com/hectorgimenez/koolo/internal/action/step"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/pather"
	"github.com/hectorgimenez/koolo/internal/town"
	"github.com/hectorgimenez/koolo/internal/ui"
	"github.com/hectorgimenez/koolo/internal/utils"
	"github.com/lxn/win"
)

// Engine is the main shop bot controller.
type Engine struct {
	cfg       *ShopBotConfig
	logger    *ShopLogger
	filter    *ClawFilter
	overlay   *OverlayServer
	ctx       *context.Status
	running   bool
	stopCh    chan struct{}
}

// NewEngine creates a new shop bot engine.
func NewEngine(cfg *ShopBotConfig, logger *ShopLogger) *Engine {
	filter := NewClawFilter(cfg.Profiles, logger)
	var overlay *OverlayServer
	if cfg.Overlay.Enabled {
		overlay = NewOverlayServer(cfg.Overlay)
	}

	return &Engine{
		cfg:     cfg,
		logger:  logger,
		filter:  filter,
		overlay: overlay,
		stopCh:  make(chan struct{}),
	}
}

// Run is the main entry point - starts the full shop bot loop.
func (e *Engine) Run() error {
	e.running = true
	defer func() { e.running = false }()

	e.logger.Info("Starting D2R Shop Bot",
		slog.String("character", e.cfg.CharacterName),
		slog.String("difficulty", e.cfg.Difficulty),
		slog.Int("refreshes", e.cfg.RefreshesPerSession),
	)

	// Start overlay if enabled
	if e.overlay != nil {
		e.overlay.SetStatus("Starting")
		if err := e.overlay.Start(); err != nil {
			e.logger.Warn("Overlay failed to start", slog.String("error", err.Error()))
		} else {
			e.logger.Info("Overlay running", slog.Int("port", e.cfg.Overlay.OverlayPort))
		}
	}

	// Print active profiles
	for _, p := range e.cfg.EnabledProfiles() {
		e.logger.Info("Active claw profile",
			slog.String("name", p.Name),
			slog.String("desc", p.Description),
		)
	}

	// Wait for game context to be available
	e.logger.Info("Waiting for game context...")
	e.ctx = context.Get()
	if e.ctx == nil {
		return fmt.Errorf("no game context available - ensure Koolo is initialized")
	}

	gameCount := 0

	for {
		select {
		case <-e.stopCh:
			e.logger.Info("Stop signal received")
			return nil
		default:
		}

		if e.cfg.GamesPerSession > 0 && gameCount >= e.cfg.GamesPerSession {
			e.logger.Info("Game limit reached", slog.Int("games", gameCount))
			break
		}

		e.setOverlayStatus("Running - Game %d", gameCount+1)

		if err := e.runShoppingSession(); err != nil {
			e.logger.LogError("session", err.Error())
			if errors.Is(err, ErrFatalShopBot) {
				return err
			}
			// Non-fatal: log and continue
			time.Sleep(2 * time.Second)
		}

		gameCount++
		e.logger.stats.GamesPlayed = gameCount

		if e.cfg.GamesPerSession > 0 && gameCount < e.cfg.GamesPerSession {
			e.logger.Info("Waiting between games",
				slog.Int("delay_seconds", e.cfg.DelayBetweenGames),
			)
			time.Sleep(time.Duration(e.cfg.DelayBetweenGames) * time.Second)
		}
	}

	return nil
}

// Stop signals the engine to stop.
func (e *Engine) Stop() {
	close(e.stopCh)
}

// runShoppingSession performs one full shopping session in the current game.
func (e *Engine) runShoppingSession() error {
	ctx := context.Get()
	if ctx == nil {
		return fmt.Errorf("no game context")
	}
	e.ctx = ctx

	ctx.RefreshGameData()

	e.logger.Info("Starting shopping session",
		slog.String("area", ctx.Data.PlayerUnit.Area.Area().Name),
		slog.Int("gold", ctx.Data.PlayerUnit.TotalPlayerGold()),
	)

	// Step 1: Ensure we're in legacy mode if configured
	if e.cfg.AutoLegacyMode {
		e.switchToLegacyIfNeeded()
	}

	// Step 2: Navigate to Harrogath
	e.setOverlayStatus("Pathing to Harrogath")
	if err := e.ensureInHarrogath(); err != nil {
		return fmt.Errorf("failed to reach Harrogath: %w", err)
	}

	// Step 3: Run the shop refresh loop
	for refresh := 0; refresh <= e.cfg.RefreshesPerSession; refresh++ {
		select {
		case <-e.stopCh:
			return nil
		default:
		}

		e.logger.stats.ShopRefreshes = refresh
		e.setOverlayStatus("Shopping - Refresh %d/%d", refresh, e.cfg.RefreshesPerSession)

		e.logger.Info("Shop refresh",
			slog.Int("refresh", refresh),
			slog.Int("total", e.cfg.RefreshesPerSession),
			slog.Int("gold", ctx.Data.PlayerUnit.TotalPlayerGold()),
		)

		// Check gold reserve
		if ctx.Data.PlayerUnit.TotalPlayerGold() < e.cfg.MinGoldReserve {
			e.logger.Warn("Gold below reserve threshold, stopping",
				slog.Int("gold", ctx.Data.PlayerUnit.TotalPlayerGold()),
				slog.Int("reserve", e.cfg.MinGoldReserve),
			)
			break
		}

		// Walk to Anya and shop
		if err := e.walkToAnyaAndShop(); err != nil {
			e.logger.LogError("shopping", err.Error())
			// Try to recover by closing menus
			step.CloseAllMenus()
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Refresh vendors if more passes remain
		if refresh < e.cfg.RefreshesPerSession {
			if err := e.refreshVendors(); err != nil {
				e.logger.LogError("refresh", err.Error())
				break
			}
		}
	}

	return nil
}

// switchToLegacyIfNeeded checks and switches to legacy mode.
func (e *Engine) switchToLegacyIfNeeded() {
	ctx := e.ctx
	if ctx == nil {
		return
	}
	ctx.RefreshGameData()

	if ctx.CharacterCfg != nil && ctx.CharacterCfg.ClassicMode && !ctx.Data.LegacyGraphics {
		e.logger.Info("Switching to legacy graphics mode")
		action.SwitchToLegacyMode()
		time.Sleep(1500 * time.Millisecond)
		ctx.RefreshGameData()
		if ctx.Data.LegacyGraphics {
			e.logger.Info("Legacy mode activated successfully")
		} else {
			e.logger.Warn("Legacy mode switch may have failed - pressing G key as fallback")
			ctx.HID.PressKey(0x47) // 'G' key - common legacy toggle
			time.Sleep(1000 * time.Millisecond)
			ctx.RefreshGameData()
		}
	}
}

// ensureInHarrogath navigates to Harrogath from wherever the player currently is.
func (e *Engine) ensureInHarrogath() error {
	ctx := context.Get()
	ctx.RefreshGameData()

	currentArea := ctx.Data.PlayerUnit.Area
	e.logger.Info("Current location",
		slog.String("area", currentArea.Area().Name),
		slog.Int("act", currentArea.Act()),
	)

	e.logger.stats.PathingAttempts++

	// Already in Harrogath
	if currentArea == area.Harrogath {
		e.logger.Info("Already in Harrogath")
		e.updateOverlayMarkers()
		return nil
	}

	// If in a town, use waypoint to get to Harrogath
	if currentArea.IsTown() {
		e.setOverlayStatus("Using waypoint to Harrogath")
		e.logger.Info("In town, waypointing to Harrogath",
			slog.String("from", currentArea.Area().Name),
		)

		if err := action.WayPoint(area.Harrogath); err != nil {
			e.logger.stats.PathingFailures++
			return fmt.Errorf("waypoint to Harrogath failed: %w", err)
		}

		ctx.RefreshGameData()
		if ctx.Data.PlayerUnit.Area == area.Harrogath {
			e.logger.Info("Arrived in Harrogath via waypoint")
			e.updateOverlayMarkers()
			return nil
		}
	}

	// If not in town, return to town first
	if !currentArea.IsTown() {
		e.logger.Info("Not in town, returning to town first")
		if err := action.ReturnTown(); err != nil {
			return fmt.Errorf("return to town failed: %w", err)
		}
		ctx.RefreshGameData()
	}

	// Now try waypoint to Harrogath
	e.setOverlayStatus("Waypointing to Harrogath")
	if err := action.WayPoint(area.Harrogath); err != nil {
		e.logger.stats.PathingFailures++
		return fmt.Errorf("waypoint to Harrogath from town failed: %w", err)
	}

	ctx.RefreshGameData()
	if ctx.Data.PlayerUnit.Area != area.Harrogath {
		e.logger.stats.PathingFailures++
		return fmt.Errorf("expected Harrogath but in %s", ctx.Data.PlayerUnit.Area.Area().Name)
	}

	e.logger.Info("Arrived in Harrogath")
	e.updateOverlayMarkers()
	return nil
}

// walkToAnyaAndShop navigates to Anya and opens her shop for claw scanning.
func (e *Engine) walkToAnyaAndShop() error {
	ctx := context.Get()
	ctx.RefreshGameData()

	e.logger.stats.PathingAttempts++

	// Calculate path to Anya before starting
	anyaPos, anyaFound := findAnyaPosition(ctx)
	if !anyaFound {
		// Use known anchor position for Anya in Harrogath
		anyaPos = data.Position{X: 5112, Y: 5120}
		e.logger.Debug("Anya not visible yet, using known anchor position")
	}

	playerPos := ctx.Data.PlayerUnit.Position
	distance := pather.DistanceFromPoint(playerPos, anyaPos)

	e.logger.LogPathingStep(
		fmt.Sprintf("Player (%d,%d)", playerPos.X, playerPos.Y),
		fmt.Sprintf("Anya (%d,%d)", anyaPos.X, anyaPos.Y),
		distance, 0, 0,
	)

	// Update overlay with Anya marker
	if e.overlay != nil {
		e.overlay.ClearMarkers()
		e.overlay.AddMarker(OverlayMarker{
			Type:   "npc",
			Label:  "Anya (Drehya)",
			WorldX: anyaPos.X,
			WorldY: anyaPos.Y,
			Color:  "#4aff4a",
			Size:   12,
			Pulsing: true,
		})
		e.overlay.AddMarker(OverlayMarker{
			Type:   "player",
			Label:  "Player",
			WorldX: playerPos.X,
			WorldY: playerPos.Y,
			Color:  "#ff4a4a",
			Size:   8,
		})
	}

	e.setOverlayStatus("Walking to Anya (distance: %d)", distance)
	e.logger.Info("Walking to Anya",
		slog.Int("distance", distance),
		slog.Int("playerX", playerPos.X),
		slog.Int("playerY", playerPos.Y),
		slog.Int("anyaX", anyaPos.X),
		slog.Int("anyaY", anyaPos.Y),
	)

	// Move to Anya
	if err := moveToAnya(ctx); err != nil {
		e.logger.stats.PathingFailures++
		return fmt.Errorf("failed walking to Anya: %w", err)
	}

	// Interact with Anya
	e.setOverlayStatus("Interacting with Anya")
	e.logger.stats.AnyaInteractions++

	if err := action.InteractNPC(npc.Drehya); err != nil {
		return fmt.Errorf("failed to interact with Anya: %w", err)
	}

	// Open trade window
	ctx.HID.KeySequence(win.VK_HOME, win.VK_DOWN, win.VK_RETURN)
	utils.Sleep(500)
	ctx.RefreshGameData()

	if !ctx.Data.OpenMenus.NPCShop {
		return fmt.Errorf("Anya's shop did not open")
	}

	e.logger.Info("Anya shop opened, scanning for claws...")
	e.setOverlayStatus("Scanning Anya's shop")

	// Scan all vendor tabs for claws
	purchased, err := e.scanShopForClaws()
	if err != nil {
		step.CloseAllMenus()
		return fmt.Errorf("error scanning shop: %w", err)
	}

	e.logger.Info("Shop scan complete",
		slog.Int("purchased", purchased),
	)

	step.CloseAllMenus()
	return nil
}

// scanShopForClaws iterates through Anya's vendor tabs looking for matching claws.
func (e *Engine) scanShopForClaws() (int, error) {
	ctx := context.Get()
	purchased := 0

	for tab := 1; tab <= 4; tab++ {
		// Switch vendor tab
		action.SwitchVendorTab(tab)
		utils.Sleep(300)
		ctx.RefreshGameData()

		vendorItems := ctx.Data.Inventory.ByLocation(item.LocationVendor)

		for _, it := range vendorItems {
			if (it.Location.Page + 1) != tab {
				continue
			}

			// Evaluate the item against our claw filters
			result := e.filter.EvaluateItem(it)

			if result.Matched {
				e.logger.Info("BUYING CLAW",
					slog.String("match", result.MatchReason),
					slog.String("profile", result.MatchedProfile),
					slog.Int("gold", ctx.Data.PlayerUnit.TotalPlayerGold()),
				)

				// Update overlay
				if e.overlay != nil {
					e.overlay.SetLastMessage(fmt.Sprintf("BUYING: %s", result.MatchReason))
					e.overlay.AddMarker(OverlayMarker{
						Type:    "item",
						Label:   fmt.Sprintf("BOUGHT: %s", string(it.Name)),
						WorldX:  ctx.Data.PlayerUnit.Position.X,
						WorldY:  ctx.Data.PlayerUnit.Position.Y,
						Color:   "#d4a74a",
						Size:    10,
						Pulsing: true,
					})
				}

				// Buy the claw
				prevGold := ctx.Data.PlayerUnit.TotalPlayerGold()
				town.BuyItem(it, 1)
				utils.Sleep(200)
				ctx.RefreshGameData()

				goldSpent := prevGold - ctx.Data.PlayerUnit.TotalPlayerGold()
				e.logger.stats.GoldSpent += goldSpent
				e.logger.stats.ClawsPurchased++
				purchased++

				e.logger.Info("Claw purchased",
					slog.Int("goldSpent", goldSpent),
					slog.Int("goldRemaining", ctx.Data.PlayerUnit.TotalPlayerGold()),
				)

				// Check gold reserve
				if ctx.Data.PlayerUnit.TotalPlayerGold() < e.cfg.MinGoldReserve {
					e.logger.Warn("Gold below reserve after purchase, stopping")
					return purchased, nil
				}
			}
		}
	}

	return purchased, nil
}

// refreshVendors refreshes Anya's shop inventory by leaving town and returning.
func (e *Engine) refreshVendors() error {
	ctx := context.Get()
	e.setOverlayStatus("Refreshing vendors")
	e.logger.Debug("Refreshing vendor inventory")

	step.CloseAllMenus()
	utils.Sleep(200)

	// Method 1: Use Anya's red portal to Nihlathak's Temple and back
	if e.tryRedPortalRefresh() {
		return nil
	}

	// Method 2: Waypoint out and back
	e.logger.Debug("Red portal refresh failed, using waypoint method")
	candidates := []area.ID{
		area.FrigidHighlands,
		area.ArreatPlateau,
		area.CrystallinePassage,
	}

	for _, dest := range candidates {
		if err := action.WayPoint(dest); err == nil {
			utils.Sleep(200)
			ctx.RefreshGameData()
			if err := action.WayPoint(area.Harrogath); err == nil {
				utils.Sleep(200)
				ctx.RefreshGameData()
				e.logger.Debug("Vendor refresh complete via waypoint")
				return nil
			}
		}
	}

	return fmt.Errorf("failed to refresh vendors via any method")
}

// tryRedPortalRefresh attempts to refresh via Anya's red portal.
func (e *Engine) tryRedPortalRefresh() bool {
	ctx := context.Get()
	ctx.RefreshGameData()

	// Move near the red portal
	portalAnchor := data.Position{X: 5116, Y: 5121}
	if err := action.MoveToCoords(portalAnchor); err != nil {
		return false
	}
	utils.Sleep(300)
	ctx.RefreshGameData()

	// Find the red portal
	redPortal, found := ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		return false
	}

	// Enter the portal
	if err := action.InteractObject(redPortal, func() bool {
		return ctx.Data.AreaData.Area == area.NihlathaksTemple
	}); err != nil {
		return false
	}

	utils.Sleep(500)
	ctx.RefreshGameData()

	// Find portal back to town
	returnPortal, found := ctx.Data.Objects.FindOne(object.PermanentTownPortal)
	if !found {
		// Move around to find it
		probes := []data.Position{
			{X: 2, Y: 0}, {X: -2, Y: 0}, {X: 0, Y: 2}, {X: 0, Y: -2},
		}
		anchor := ctx.Data.PlayerUnit.Position
		for _, off := range probes {
			action.MoveToCoords(data.Position{X: anchor.X + off.X, Y: anchor.Y + off.Y})
			utils.Sleep(200)
			ctx.RefreshGameData()
			if rp, ok := ctx.Data.Objects.FindOne(object.PermanentTownPortal); ok {
				returnPortal = rp
				found = true
				break
			}
		}
	}

	if !found {
		e.logger.Warn("Return portal not found in Nihlathak's Temple")
		return false
	}

	// Wait cooldown before re-entering portal
	time.Sleep(2 * time.Second)

	if err := action.InteractObject(returnPortal, func() bool {
		return ctx.Data.AreaData.Area == area.Harrogath
	}); err != nil {
		return false
	}

	utils.Sleep(300)
	ctx.RefreshGameData()
	e.logger.Debug("Vendor refresh complete via red portal")
	return true
}

// findAnyaPosition locates Anya in the current game data.
func findAnyaPosition(ctx *context.Status) (data.Position, bool) {
	// Try monsters list first (Anya is sometimes classified as a monster NPC)
	if m, found := ctx.Data.Monsters.FindOne(npc.Drehya, data.MonsterTypeNone); found {
		return m.Position, true
	}

	// Try NPCs list
	if n, found := ctx.Data.NPCs.FindOne(npc.Drehya); found && len(n.Positions) > 0 {
		return n.Positions[0], true
	}

	return data.Position{}, false
}

// moveToAnya navigates to Anya with fallback positions.
func moveToAnya(ctx *context.Status) error {
	// Try to find Anya directly
	if m, found := ctx.Data.Monsters.FindOne(npc.Drehya, data.MonsterTypeNone); found {
		return action.MoveToCoords(m.Position)
	}

	// Use known anchor position near Anya
	anchor := data.Position{X: 5112, Y: 5120}
	if err := action.MoveToCoords(anchor); err != nil {
		return err
	}

	// Check if we can see Anya now
	ctx.RefreshGameData()
	if m, found := ctx.Data.Monsters.FindOne(npc.Drehya, data.MonsterTypeNone); found {
		return action.MoveToCoords(m.Position)
	}

	// Try nearby positions
	nearbyPositions := []data.Position{
		{X: 5116, Y: 5121},
		{X: 5108, Y: 5118},
		{X: 5120, Y: 5124},
	}
	for _, pos := range nearbyPositions {
		if err := action.MoveToCoords(pos); err == nil {
			ctx.RefreshGameData()
			if _, found := ctx.Data.Monsters.FindOne(npc.Drehya, data.MonsterTypeNone); found {
				return nil
			}
		}
	}

	return nil // Best effort - we're near enough for NPC interaction to work
}

// updateOverlayMarkers refreshes all markers on the overlay.
func (e *Engine) updateOverlayMarkers() {
	if e.overlay == nil {
		return
	}

	ctx := context.Get()
	if ctx == nil {
		return
	}
	ctx.RefreshGameData()

	e.overlay.ClearMarkers()

	// Mark waypoints
	if e.cfg.Overlay.ShowWaypoints {
		for _, obj := range ctx.Data.Objects {
			if obj.IsWaypoint() {
				e.overlay.AddMarker(OverlayMarker{
					Type:   "waypoint",
					Label:  "Waypoint",
					WorldX: obj.Position.X,
					WorldY: obj.Position.Y,
					Color:  "#4a9dff",
					Size:   e.cfg.Overlay.MarkerSize,
				})
			}
		}
	}

	// Mark NPCs
	if e.cfg.Overlay.ShowNPCs {
		npcList := []struct {
			id   npc.ID
			name string
		}{
			{npc.Drehya, "Anya"},
			{npc.Malah, "Malah"},
			{npc.Larzuk, "Larzuk"},
			{npc.QualKehk, "Qual-Kehk"},
		}
		for _, n := range npcList {
			if m, found := ctx.Data.Monsters.FindOne(n.id, data.MonsterTypeNone); found {
				color := "#4aff4a"
				size := e.cfg.Overlay.MarkerSize
				pulsing := false
				if n.id == npc.Drehya {
					color = "#ff4aff"
					size = 12
					pulsing = true
				}
				e.overlay.AddMarker(OverlayMarker{
					Type:    "npc",
					Label:   n.name,
					WorldX:  m.Position.X,
					WorldY:  m.Position.Y,
					Color:   color,
					Size:    size,
					Pulsing: pulsing,
				})
			}
		}
	}

	// Mark the red portal
	if portal, found := ctx.Data.Objects.FindOne(object.PermanentTownPortal); found {
		e.overlay.AddMarker(OverlayMarker{
			Type:   "waypoint",
			Label:  "Red Portal",
			WorldX: portal.Position.X,
			WorldY: portal.Position.Y,
			Color:  "#ff0000",
			Size:   10,
		})
	}

	// Update stats
	e.overlay.UpdateState(OverlayState{
		Markers:       e.overlay.state.Markers,
		PlayerPos:     ctx.Data.PlayerUnit.Position,
		PlayerArea:    ctx.Data.PlayerUnit.Area.Area().Name,
		TargetNPC:     "Anya",
		ShopRefreshes: e.logger.stats.ShopRefreshes,
		ClawsScanned:  e.logger.stats.ClawsScanned,
		ClawsFound:    e.logger.stats.ClawsMatched,
		GoldCurrent:   ctx.Data.PlayerUnit.TotalPlayerGold(),
		Status:        e.overlay.state.Status,
	})
}

func (e *Engine) setOverlayStatus(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if e.overlay != nil {
		e.overlay.SetStatus(msg)
	}
	e.logger.Debug("Status: " + msg)
}

// GameCoordsToScreenCords converts game coordinates to screen coordinates.
// This delegates to the existing UI package.
func GameCoordsToScreenCords(x, y int) (int, int) {
	return ui.GameCoordsToScreenCords(x, y)
}

// DistanceFromPoint calculates distance between two positions.
func DistanceFromPoint(a, b data.Position) int {
	return pather.DistanceFromPoint(a, b)
}

// SwitchVendorTab switches to the specified vendor tab (uses the existing action).
func SwitchVendorTab(tab int) {
	action.SwitchVendorTab(tab)
}

// InteractNPC wraps the existing NPC interaction.
func InteractNPC(id npc.ID) error {
	return action.InteractNPC(id)
}

var ErrFatalShopBot = errors.New("fatal shop bot error")
