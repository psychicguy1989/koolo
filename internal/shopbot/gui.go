package shopbot

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// GUIState represents the full state of the GUI application.
type GUIState struct {
	mu sync.RWMutex

	// Process detection
	DetectedProcesses []D2RProcess `json:"detectedProcesses"`
	SelectedProcess   *D2RProcess  `json:"selectedProcess"`

	// Game state
	GameState     GameStateInfo `json:"gameState"`
	IsAttached    bool          `json:"isAttached"`
	InjectorReady bool          `json:"injectorReady"`

	// Bot status
	BotRunning   bool   `json:"botRunning"`
	BotStatus    string `json:"botStatus"`
	BotPhase     string `json:"botPhase"` // "idle", "testing", "shopping", "paused", "break"
	BreakEndTime string `json:"breakEndTime"`

	// Shopping stats
	ShopRefreshes  int `json:"shopRefreshes"`
	ClawsScanned   int `json:"clawsScanned"`
	ClawsMatched   int `json:"clawsMatched"`
	ClawsPurchased int `json:"clawsPurchased"`
	GoldSpent      int `json:"goldSpent"`

	// Log messages
	LogMessages []LogEntry `json:"logMessages"`

	// Break config
	BreakEnabled      bool `json:"breakEnabled"`
	BreakMinutes      int  `json:"breakMinutes"`
	BreakRandomize    bool `json:"breakRandomize"`
	BreakRandomRange  int  `json:"breakRandomRange"` // +/- this many minutes

	// Config
	Config *ShopBotConfig `json:"config"`
}

// LogEntry is a single log message with timestamp and level.
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// GUIServer serves the shopbot GUI and handles API requests.
type GUIServer struct {
	state    *GUIState
	engine   *Engine
	standCtx *StandaloneContext
	cfg      *ShopBotConfig
	logger   *ShopLogger
	port     int
}

// NewGUIServer creates a new GUI server.
func NewGUIServer(cfg *ShopBotConfig, logger *ShopLogger, port int) *GUIServer {
	return &GUIServer{
		state: &GUIState{
			BotPhase:  "idle",
			BotStatus: "Ready - Detect D2R clients to begin",
			Config:    cfg,
		},
		cfg:    cfg,
		logger: logger,
		port:   port,
	}
}

// AddLog adds a log entry visible in the GUI.
func (g *GUIServer) AddLog(level, message string) {
	g.state.mu.Lock()
	defer g.state.mu.Unlock()

	entry := LogEntry{
		Time:    time.Now().Format("15:04:05"),
		Level:   level,
		Message: message,
	}
	g.state.LogMessages = append(g.state.LogMessages, entry)

	// Keep last 500 messages
	if len(g.state.LogMessages) > 500 {
		g.state.LogMessages = g.state.LogMessages[len(g.state.LogMessages)-500:]
	}
}

// Start launches the GUI HTTP server.
func (g *GUIServer) Start() error {
	mux := http.NewServeMux()

	// Main GUI page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, guiHTML)
	})

	// API: Get current state
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		g.state.mu.RLock()
		defer g.state.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(g.state)
	})

	// API: Detect D2R processes
	mux.HandleFunc("/api/detect", func(w http.ResponseWriter, r *http.Request) {
		g.handleDetectProcesses(w, r)
	})

	// API: Attach to selected process
	mux.HandleFunc("/api/attach", func(w http.ResponseWriter, r *http.Request) {
		g.handleAttach(w, r)
	})

	// API: Run diagnostic test
	mux.HandleFunc("/api/test", func(w http.ResponseWriter, r *http.Request) {
		g.handleTest(w, r)
	})

	// API: Start shopping
	mux.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		g.handleStartShopping(w, r)
	})

	// API: Stop shopping
	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		g.handleStopShopping(w, r)
	})

	// API: Configure in-game settings
	mux.HandleFunc("/api/configure-ingame", func(w http.ResponseWriter, r *http.Request) {
		g.handleConfigureInGame(w, r)
	})

	// API: Save break settings
	mux.HandleFunc("/api/save-break", func(w http.ResponseWriter, r *http.Request) {
		g.handleSaveBreak(w, r)
	})

	addr := fmt.Sprintf(":%d", g.port)
	g.AddLog("INFO", fmt.Sprintf("GUI server starting on http://localhost:%d", g.port))

	go http.ListenAndServe(addr, mux)
	return nil
}

// handleDetectProcesses scans for running D2R clients.
func (g *GUIServer) handleDetectProcesses(w http.ResponseWriter, r *http.Request) {
	g.AddLog("INFO", "Scanning for D2R processes...")

	processes, err := FindAllD2RProcesses()
	if err != nil {
		g.AddLog("ERROR", fmt.Sprintf("Failed to scan processes: %v", err))
		http.Error(w, err.Error(), 500)
		return
	}

	g.state.mu.Lock()
	g.state.DetectedProcesses = processes
	g.state.mu.Unlock()

	if len(processes) == 0 {
		g.AddLog("WARN", "No D2R processes found. Start Diablo 2 Resurrected first.")
	} else {
		g.AddLog("INFO", fmt.Sprintf("Found %d D2R process(es)", len(processes)))
		for i, p := range processes {
			g.AddLog("INFO", fmt.Sprintf("  [%d] PID %d - %q", i+1, p.PID, p.WindowTitle))
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(processes)
}

// handleAttach connects to the selected D2R process.
func (g *GUIServer) handleAttach(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PID uint32 `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}

	// Find the process
	g.state.mu.RLock()
	var proc *D2RProcess
	for _, p := range g.state.DetectedProcesses {
		if p.PID == req.PID {
			proc = &p
			break
		}
	}
	g.state.mu.RUnlock()

	if proc == nil {
		g.AddLog("ERROR", fmt.Sprintf("Process with PID %d not found", req.PID))
		http.Error(w, "process not found", 404)
		return
	}

	g.AddLog("INFO", fmt.Sprintf("Attaching to D2R process PID %d...", proc.PID))

	// Create standalone context
	standCtx, err := NewStandaloneContext(*proc, g.logger.Logger)
	if err != nil {
		g.AddLog("ERROR", fmt.Sprintf("Failed to attach: %v", err))
		http.Error(w, err.Error(), 500)
		return
	}

	// Load injector
	if err := standCtx.LoadInjector(); err != nil {
		g.AddLog("ERROR", fmt.Sprintf("Failed to load injector: %v", err))
		http.Error(w, err.Error(), 500)
		return
	}

	g.standCtx = standCtx

	// Try to detect character name
	charName := standCtx.GetCharacterName()
	if charName != "" {
		proc.CharacterName = charName
		g.AddLog("INFO", fmt.Sprintf("Character detected: %s", charName))
	}

	g.state.mu.Lock()
	g.state.SelectedProcess = proc
	g.state.IsAttached = true
	g.state.InjectorReady = true
	g.state.BotStatus = "Attached - Run tests or start shopping"
	g.state.mu.Unlock()

	g.AddLog("INFO", "Successfully attached to D2R process")
	g.AddLog("INFO", "Memory reader: READY")
	g.AddLog("INFO", "Memory injector: READY")
	g.AddLog("INFO", "HID input: READY")
	g.AddLog("INFO", "Pathfinder: READY")
	g.AddLog("INFO", "You can now run tests or start shopping")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "attached"})
}

// handleTest runs the step-by-step diagnostic test.
func (g *GUIServer) handleTest(w http.ResponseWriter, r *http.Request) {
	if g.standCtx == nil || !g.standCtx.Attached {
		g.AddLog("ERROR", "Not attached to any D2R process")
		http.Error(w, "not attached", 400)
		return
	}

	g.state.mu.Lock()
	g.state.BotPhase = "testing"
	g.state.BotStatus = "Running diagnostics..."
	g.state.mu.Unlock()

	g.AddLog("INFO", "========================================")
	g.AddLog("INFO", "  RUNNING DIAGNOSTIC TESTS")
	g.AddLog("INFO", "========================================")

	// Test 1: Read game state
	g.AddLog("INFO", "[TEST 1] Reading game state from memory...")
	info := g.standCtx.DetectGameState()

	g.state.mu.Lock()
	g.state.GameState = info
	g.state.mu.Unlock()

	if !info.InGame {
		if info.InCharSelect {
			g.AddLog("WARN", "Character is at selection screen - join a game first")
		} else if info.InLobby {
			g.AddLog("WARN", "Character is in lobby - join a game first")
		} else {
			g.AddLog("WARN", "Character does not appear to be in-game")
		}
		g.state.mu.Lock()
		g.state.BotPhase = "idle"
		g.state.BotStatus = "Test failed - character not in game"
		g.state.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
		return
	}

	g.AddLog("INFO", fmt.Sprintf("  In-game: YES"))
	g.AddLog("INFO", fmt.Sprintf("  Area: %s (Act %d)", info.Area, info.Act))
	g.AddLog("INFO", fmt.Sprintf("  Town: %v", info.IsTown))
	g.AddLog("INFO", fmt.Sprintf("  Position: %d, %d", info.PlayerX, info.PlayerY))
	g.AddLog("INFO", fmt.Sprintf("  HP: %d%%", info.PlayerHP))
	g.AddLog("INFO", fmt.Sprintf("  Gold: %d", info.PlayerGold))
	g.AddLog("INFO", fmt.Sprintf("  Legacy graphics: %v", info.LegacyGraphics))

	// Test 2: Check if in Harrogath
	g.AddLog("INFO", "[TEST 2] Checking location...")
	if info.Area == "Harrogath" {
		g.AddLog("INFO", "  Currently in Harrogath - GOOD")
	} else if info.IsTown {
		g.AddLog("INFO", fmt.Sprintf("  In %s - will need to waypoint to Harrogath", info.Area))
	} else {
		g.AddLog("WARN", fmt.Sprintf("  Not in town (%s) - will need to return to town first", info.Area))
	}

	// Test 3: Detect Anya
	g.AddLog("INFO", "[TEST 3] Detecting Anya (Drehya)...")
	if info.AnyaVisible {
		g.AddLog("INFO", fmt.Sprintf("  Anya FOUND at (%d, %d), distance: %d tiles", info.AnyaX, info.AnyaY, info.AnyaDistance))
	} else {
		if info.Area == "Harrogath" {
			g.AddLog("WARN", "  Anya NOT visible from current position")
			g.AddLog("INFO", "  She may be at her known position (5112, 5120)")
			g.AddLog("INFO", "  Check: Have you completed the Rescue Anya quest?")
			g.AddLog("INFO", "  If the quest is not done, Anya won't be in town.")
			g.AddLog("ERROR", "  --> If quest is incomplete: Complete 'Prison of Ice' quest first!")
		} else {
			g.AddLog("INFO", "  Anya not visible (not in Harrogath yet)")
		}
	}

	// Test 4: Detect waypoint
	g.AddLog("INFO", "[TEST 4] Detecting waypoint...")
	if info.WaypointNearby {
		g.AddLog("INFO", "  Waypoint detected nearby - GOOD")
	} else {
		g.AddLog("INFO", "  No waypoint in visible range")
	}

	// Test 5: Detect red portal
	g.AddLog("INFO", "[TEST 5] Detecting Anya's red portal...")
	if info.RedPortalFound {
		g.AddLog("INFO", "  Red portal detected - GOOD (used for vendor refresh)")
	} else {
		g.AddLog("INFO", "  Red portal not visible (may need to be closer)")
	}

	// Test 6: Check menus
	g.AddLog("INFO", "[TEST 6] Checking menu state...")
	if info.ShopOpen {
		g.AddLog("INFO", "  Shop window is OPEN")
	} else if info.NPCInteract {
		g.AddLog("INFO", "  NPC dialog is open")
	} else if info.MenuOpen {
		g.AddLog("INFO", "  Some menu is open")
	} else {
		g.AddLog("INFO", "  No menus open - GOOD")
	}

	// Summary
	g.AddLog("INFO", "========================================")
	g.AddLog("INFO", "  DIAGNOSTIC SUMMARY")
	g.AddLog("INFO", "========================================")

	ready := true
	if !info.InGame {
		g.AddLog("ERROR", "  NOT READY: Character must be in-game")
		ready = false
	}
	if info.Area != "Harrogath" {
		g.AddLog("WARN", "  Will navigate to Harrogath via waypoint")
	}
	if !info.AnyaVisible && info.Area == "Harrogath" {
		g.AddLog("WARN", "  Anya not visible - quest may not be complete")
		g.AddLog("ERROR", "  --> Complete 'Prison of Ice' quest and talk to Anya!")
	}

	if ready {
		g.AddLog("INFO", "  RESULT: Ready to shop! Click 'Start Shopping' when ready.")
		g.state.mu.Lock()
		g.state.BotStatus = "Tests passed - Ready to shop"
		g.state.mu.Unlock()
	} else {
		g.state.mu.Lock()
		g.state.BotStatus = "Tests found issues - see log"
		g.state.mu.Unlock()
	}

	g.state.mu.Lock()
	g.state.BotPhase = "idle"
	g.state.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleStartShopping begins the shopping loop.
func (g *GUIServer) handleStartShopping(w http.ResponseWriter, r *http.Request) {
	if g.standCtx == nil || !g.standCtx.Attached {
		g.AddLog("ERROR", "Not attached to any D2R process")
		http.Error(w, "not attached", 400)
		return
	}

	g.AddLog("INFO", "Starting shopping bot...")

	g.state.mu.Lock()
	g.state.BotRunning = true
	g.state.BotPhase = "shopping"
	g.state.BotStatus = "Shopping in progress..."
	g.state.mu.Unlock()

	// Create engine and wire standalone context
	engine := NewEngine(g.cfg, g.logger)
	if g.standCtx != nil {
		engine.SetStandaloneContext(g.standCtx.Ctx)
	}
	g.engine = engine

	go func() {
		if err := engine.Run(); err != nil {
			g.AddLog("ERROR", fmt.Sprintf("Shopping error: %v", err))
		}
		g.state.mu.Lock()
		g.state.BotRunning = false
		g.state.BotPhase = "idle"
		g.state.BotStatus = "Shopping stopped"
		g.state.mu.Unlock()
		g.AddLog("INFO", "Shopping session ended")
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handleStopShopping stops the shopping loop.
func (g *GUIServer) handleStopShopping(w http.ResponseWriter, r *http.Request) {
	if g.engine != nil {
		g.engine.Stop()
		g.AddLog("INFO", "Stop signal sent to shopping bot")
	}

	g.state.mu.Lock()
	g.state.BotRunning = false
	g.state.BotPhase = "idle"
	g.state.BotStatus = "Stopped"
	g.state.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

// handleConfigureInGame configures D2R settings via the in-game menu.
func (g *GUIServer) handleConfigureInGame(w http.ResponseWriter, r *http.Request) {
	if g.standCtx == nil || !g.standCtx.Attached {
		http.Error(w, "not attached", 400)
		return
	}

	g.AddLog("INFO", "Configuring in-game settings...")
	g.AddLog("INFO", "Step 1: Pressing ESC to open game menu...")

	go func() {
		configurator := NewInGameConfigurator(g.standCtx, g.logger, g)
		if err := configurator.ConfigureAll(); err != nil {
			g.AddLog("ERROR", fmt.Sprintf("Configuration failed: %v", err))
		} else {
			g.AddLog("INFO", "In-game configuration complete!")
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "configuring"})
}

// handleSaveBreak saves break settings.
func (g *GUIServer) handleSaveBreak(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Enabled     bool `json:"enabled"`
		Minutes     int  `json:"minutes"`
		Randomize   bool `json:"randomize"`
		RandomRange int  `json:"randomRange"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", 400)
		return
	}

	g.state.mu.Lock()
	g.state.BreakEnabled = req.Enabled
	g.state.BreakMinutes = req.Minutes
	g.state.BreakRandomize = req.Randomize
	g.state.BreakRandomRange = req.RandomRange
	g.state.mu.Unlock()

	g.AddLog("INFO", fmt.Sprintf("Break settings saved: %d min (random: %v, +/-%d min)",
		req.Minutes, req.Randomize, req.RandomRange))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
}

// ShouldTakeBreak checks if it's time for a break and calculates duration.
func (g *GUIServer) ShouldTakeBreak() (bool, time.Duration) {
	g.state.mu.RLock()
	defer g.state.mu.RUnlock()

	if !g.state.BreakEnabled || g.state.BreakMinutes <= 0 {
		return false, 0
	}

	minutes := g.state.BreakMinutes
	if g.state.BreakRandomize && g.state.BreakRandomRange > 0 {
		offset := rand.Intn(g.state.BreakRandomRange*2+1) - g.state.BreakRandomRange
		minutes += offset
		if minutes < 1 {
			minutes = 1
		}
	}

	return true, time.Duration(minutes) * time.Minute
}
