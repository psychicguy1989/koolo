package shopbot

import (
	"fmt"
	"log/slog"

	"github.com/hectorgimenez/d2go/pkg/data"
	"github.com/hectorgimenez/d2go/pkg/data/npc"
	"github.com/hectorgimenez/d2go/pkg/data/object"
	"github.com/hectorgimenez/koolo/internal/config"
	"github.com/hectorgimenez/koolo/internal/context"
	"github.com/hectorgimenez/koolo/internal/game"
	"github.com/hectorgimenez/koolo/internal/health"
	"github.com/hectorgimenez/koolo/internal/pather"
)

// StandaloneContext holds all the components needed to run independently.
type StandaloneContext struct {
	Process        D2RProcess
	MemoryReader   *game.MemoryReader
	MemoryInjector *game.MemoryInjector
	HID            *game.HID
	PathFinder     *pather.PathFinder
	GameData       *game.Data
	CharacterCfg   *config.CharacterCfg
	Ctx            *context.Status
	Logger         *slog.Logger
	Attached       bool
}

// NewStandaloneContext creates a fully independent game connection to a D2R process.
// This does NOT require Koolo to be running.
func NewStandaloneContext(proc D2RProcess, logger *slog.Logger) (*StandaloneContext, error) {
	sc := &StandaloneContext{
		Process: proc,
		Logger:  logger,
	}

	logger.Info("Initializing standalone context",
		slog.Int("pid", int(proc.PID)),
		slog.String("window", proc.WindowTitle),
	)

	// Step 1: Create a minimal CharacterCfg for the shopbot
	charCfg := createShopBotCharacterCfg()
	sc.CharacterCfg = charCfg

	// Step 2: Create MemoryReader
	logger.Info("Creating memory reader...")
	gr, err := game.NewGameReader(charCfg, "shopbot", proc.PID, proc.HWND, logger)
	if err != nil {
		return nil, fmt.Errorf("creating memory reader: %w", err)
	}
	sc.MemoryReader = gr
	logger.Info("Memory reader created successfully")

	// Step 3: Create MemoryInjector
	logger.Info("Creating memory injector...")
	gi, err := game.InjectorInit(logger, proc.PID)
	if err != nil {
		return nil, fmt.Errorf("creating memory injector: %w", err)
	}
	sc.MemoryInjector = gi
	logger.Info("Memory injector created successfully")

	// Step 4: Create HID (combines reader + injector)
	logger.Info("Creating HID input system...")
	hid := game.NewHID(gr, gi)
	sc.HID = hid
	logger.Info("HID input system created")

	// Step 5: Create game context
	logger.Info("Creating game context...")
	ctx := context.NewContext("shopbot")
	sc.Ctx = ctx

	// Step 6: Wire everything into the context
	ctx.CharacterCfg = charCfg
	ctx.HID = hid
	ctx.Logger = logger
	ctx.GameReader = gr
	ctx.MemoryInjector = gi
	ctx.Manager = game.NewGameManager(gr, hid, "shopbot")
	ctx.PacketSender = game.NewPacketSender(gr.Process)

	// Step 7: Initialize game data
	sc.GameData = ctx.Data

	// Step 8: Create PathFinder
	logger.Info("Creating pathfinder...")
	pf := pather.NewPathFinder(gr, ctx.Data, hid, charCfg)
	pf.SetPacketSender(ctx.PacketSender)
	sc.PathFinder = pf
	ctx.PathFinder = pf
	logger.Info("Pathfinder created")

	// Step 9: Create health/belt managers
	bm := health.NewBeltManager(ctx.Data, hid, logger, "shopbot")
	ctx.BeltManager = bm
	hm := health.NewHealthManager(bm, ctx.Data)
	ctx.HealthManager = hm

	sc.Attached = true
	logger.Info("Standalone context fully initialized")

	return sc, nil
}

// LoadInjector activates the memory injection hooks (hooks USER32.dll functions).
func (sc *StandaloneContext) LoadInjector() error {
	sc.Logger.Info("Loading memory injector (hooking USER32.dll)...")
	if err := sc.MemoryInjector.Load(); err != nil {
		return fmt.Errorf("loading memory injector: %w", err)
	}
	sc.Logger.Info("Memory injector loaded - input injection active")
	return nil
}

// UnloadInjector restores original memory and deactivates hooks.
func (sc *StandaloneContext) UnloadInjector() {
	if sc.MemoryInjector != nil {
		sc.MemoryInjector.RestoreMemory()
		sc.Logger.Info("Memory injector unloaded")
	}
}

// RefreshGameData reads fresh game state from memory.
func (sc *StandaloneContext) RefreshGameData() {
	if sc.Ctx != nil {
		sc.Ctx.RefreshGameData()
	}
}

// GetCharacterName reads the character name from the game.
func (sc *StandaloneContext) GetCharacterName() string {
	if sc.MemoryReader == nil {
		return ""
	}
	return sc.MemoryReader.GameReader.GetSelectedCharacterName()
}

// IsInGame checks if the player is currently in a game.
func (sc *StandaloneContext) IsInGame() bool {
	if sc.Ctx == nil || sc.Ctx.Manager == nil {
		return false
	}
	return sc.Ctx.Manager.InGame()
}

// Close cleans up all resources.
func (sc *StandaloneContext) Close() {
	sc.UnloadInjector()
	sc.Attached = false
	sc.Logger.Info("Standalone context closed")
}

// createShopBotCharacterCfg creates a minimal character config for the shop bot.
func createShopBotCharacterCfg() *config.CharacterCfg {
	cfg := &config.CharacterCfg{
		ClassicMode: true, // We want legacy mode for consistent coordinates
	}

	// Set common defaults
	cfg.Character.Class = "assassin"
	cfg.Health.ChickenAt = 0       // Don't chicken - we're in town
	cfg.Health.HealingPotionAt = 0 // Don't use potions
	cfg.Health.ManaPotionAt = 0

	// Inventory lock - protect everything
	cfg.Inventory.InventoryLock = [][]int{
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		{0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
	}

	return cfg
}

// DetectGameState reads the current game state and returns diagnostic info.
type GameStateInfo struct {
	InGame         bool   `json:"inGame"`
	InCharSelect   bool   `json:"inCharSelect"`
	InLobby        bool   `json:"inLobby"`
	LegacyGraphics bool   `json:"legacyGraphics"`
	Area           string `json:"area"`
	AreaID         int    `json:"areaId"`
	Act            int    `json:"act"`
	IsTown         bool   `json:"isTown"`
	PlayerX        int    `json:"playerX"`
	PlayerY        int    `json:"playerY"`
	PlayerHP       int    `json:"playerHp"`
	PlayerGold     int    `json:"playerGold"`
	CharacterName  string `json:"characterName"`
	AnyaVisible    bool   `json:"anyaVisible"`
	AnyaX          int    `json:"anyaX"`
	AnyaY          int    `json:"anyaY"`
	AnyaDistance   int    `json:"anyaDistance"`
	WaypointNearby bool   `json:"waypointNearby"`
	RedPortalFound bool   `json:"redPortalFound"`
	ShopOpen       bool   `json:"shopOpen"`
	NPCInteract    bool   `json:"npcInteract"`
	MenuOpen       bool   `json:"menuOpen"`
}

// DetectGameState reads all relevant game state for diagnostic display.
func (sc *StandaloneContext) DetectGameState() GameStateInfo {
	info := GameStateInfo{}

	if sc.MemoryReader == nil || sc.Ctx == nil {
		return info
	}

	// Check basic states
	info.InGame = sc.IsInGame()
	info.InCharSelect = sc.MemoryReader.IsInCharacterSelectionScreen()
	info.InLobby = sc.MemoryReader.IsInLobby()

	if !info.InGame {
		return info
	}

	// Refresh game data
	sc.RefreshGameData()
	d := sc.Ctx.Data

	info.LegacyGraphics = d.LegacyGraphics
	info.Area = d.PlayerUnit.Area.Area().Name
	info.AreaID = int(d.PlayerUnit.Area)
	info.Act = d.PlayerUnit.Area.Act()
	info.IsTown = d.PlayerUnit.Area.IsTown()
	info.PlayerX = d.PlayerUnit.Position.X
	info.PlayerY = d.PlayerUnit.Position.Y
	info.PlayerHP = d.PlayerUnit.HPPercent()
	info.PlayerGold = d.PlayerUnit.TotalPlayerGold()

	// Check menus
	info.ShopOpen = d.OpenMenus.NPCShop
	info.NPCInteract = d.OpenMenus.NPCInteract
	info.MenuOpen = d.OpenMenus.IsMenuOpen()

	// Find Anya
	if m, found := d.Monsters.FindOne(npc.Drehya, data.MonsterTypeNone); found {
		info.AnyaVisible = true
		info.AnyaX = m.Position.X
		info.AnyaY = m.Position.Y
		info.AnyaDistance = pather.DistanceFromPoint(d.PlayerUnit.Position, m.Position)
	}

	// Check waypoint
	for _, obj := range d.Objects {
		if obj.IsWaypoint() {
			info.WaypointNearby = true
			break
		}
	}

	// Check red portal
	if _, found := d.Objects.FindOne(object.PermanentTownPortal); found {
		info.RedPortalFound = true
	}

	return info
}

