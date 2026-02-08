package shopbot

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hectorgimenez/d2go/pkg/data"
)

// OverlayMarker is a visual marker to be drawn on the overlay.
type OverlayMarker struct {
	Type     string  `json:"type"`     // "waypoint", "npc", "player", "path", "item"
	Label    string  `json:"label"`
	X        int     `json:"x"`        // Screen X
	Y        int     `json:"y"`        // Screen Y
	WorldX   int     `json:"worldX"`   // Game world X
	WorldY   int     `json:"worldY"`   // Game world Y
	Color    string  `json:"color"`    // CSS color
	Size     int     `json:"size"`
	Pulsing  bool    `json:"pulsing"`  // Animate the marker
}

// OverlayState is the full state sent to the overlay client.
type OverlayState struct {
	Markers       []OverlayMarker `json:"markers"`
	PlayerPos     data.Position   `json:"playerPos"`
	PlayerArea    string          `json:"playerArea"`
	TargetNPC     string          `json:"targetNpc"`
	PathSteps     int             `json:"pathSteps"`
	PathDistance  int             `json:"pathDistance"`
	Status        string          `json:"status"`
	ShopRefreshes int             `json:"shopRefreshes"`
	ClawsScanned  int             `json:"clawsScanned"`
	ClawsFound    int             `json:"clawsFound"`
	GoldCurrent   int             `json:"goldCurrent"`
	LastMessage   string          `json:"lastMessage"`
	Uptime        string          `json:"uptime"`
}

// OverlayServer serves a real-time web overlay for game state visualization.
type OverlayServer struct {
	mu      sync.RWMutex
	state   OverlayState
	cfg     OverlayConfig
	started time.Time
}

// NewOverlayServer creates a new overlay server.
func NewOverlayServer(cfg OverlayConfig) *OverlayServer {
	return &OverlayServer{
		cfg:     cfg,
		started: time.Now(),
		state: OverlayState{
			Status: "initializing",
		},
	}
}

// UpdateState replaces the overlay state atomically.
func (o *OverlayServer) UpdateState(state OverlayState) {
	o.mu.Lock()
	defer o.mu.Unlock()
	state.Uptime = time.Since(o.started).Round(time.Second).String()
	o.state = state
}

// AddMarker adds a marker to the current state.
func (o *OverlayServer) AddMarker(m OverlayMarker) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.Markers = append(o.state.Markers, m)
}

// ClearMarkers removes all markers.
func (o *OverlayServer) ClearMarkers() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.Markers = nil
}

// SetStatus updates the status message.
func (o *OverlayServer) SetStatus(status string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.Status = status
}

// SetLastMessage updates the last message.
func (o *OverlayServer) SetLastMessage(msg string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.state.LastMessage = msg
}

// Start launches the HTTP server for the overlay.
func (o *OverlayServer) Start() error {
	if !o.cfg.Enabled {
		return nil
	}

	mux := http.NewServeMux()

	// Serve the overlay HTML page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, overlayHTML)
	})

	// API endpoint for state
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		o.mu.RLock()
		defer o.mu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		json.NewEncoder(w).Encode(o.state)
	})

	addr := fmt.Sprintf(":%d", o.cfg.OverlayPort)
	go http.ListenAndServe(addr, mux)

	return nil
}

const overlayHTML = `<!DOCTYPE html>
<html>
<head>
<title>D2R Shop Bot Overlay</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0a0a0a; color: #e0e0e0; font-family: 'Consolas', 'Monaco', monospace; font-size: 13px; }
.container { display: flex; height: 100vh; }
.sidebar { width: 320px; background: #111; border-right: 1px solid #333; padding: 12px; overflow-y: auto; }
.main { flex: 1; position: relative; display: flex; flex-direction: column; }
.canvas-wrap { flex: 1; position: relative; }
canvas { width: 100%; height: 100%; }
h1 { color: #d4a74a; font-size: 16px; margin-bottom: 8px; border-bottom: 1px solid #333; padding-bottom: 6px; }
h2 { color: #888; font-size: 12px; text-transform: uppercase; margin: 12px 0 6px; }
.stat { display: flex; justify-content: space-between; padding: 3px 0; }
.stat-label { color: #888; }
.stat-value { color: #fff; font-weight: bold; }
.stat-value.gold { color: #d4a74a; }
.stat-value.match { color: #4aff4a; }
.stat-value.error { color: #ff4a4a; }
.status { padding: 8px; background: #1a1a1a; border: 1px solid #333; border-radius: 4px; margin-bottom: 8px; text-align: center; }
.status.running { border-color: #4aff4a; color: #4aff4a; }
.status.shopping { border-color: #d4a74a; color: #d4a74a; }
.status.pathing { border-color: #4a9dff; color: #4a9dff; }
.status.error { border-color: #ff4a4a; color: #ff4a4a; }
.marker-list { max-height: 200px; overflow-y: auto; }
.marker-item { padding: 3px 6px; border-left: 3px solid transparent; margin: 2px 0; font-size: 11px; }
.marker-item.waypoint { border-color: #4a9dff; }
.marker-item.npc { border-color: #4aff4a; }
.marker-item.player { border-color: #ff4a4a; }
.marker-item.item { border-color: #d4a74a; }
.log { background: #0a0a0a; border: 1px solid #222; border-radius: 4px; padding: 6px; max-height: 150px; overflow-y: auto; font-size: 11px; color: #888; }
@keyframes pulse { 0%,100% { opacity: 1; } 50% { opacity: 0.4; } }
.pulsing { animation: pulse 1.5s infinite; }
</style>
</head>
<body>
<div class="container">
  <div class="sidebar">
    <h1>D2R SHOP BOT</h1>
    <div id="status" class="status">Connecting...</div>

    <h2>Session Stats</h2>
    <div class="stat"><span class="stat-label">Uptime</span><span id="uptime" class="stat-value">-</span></div>
    <div class="stat"><span class="stat-label">Area</span><span id="area" class="stat-value">-</span></div>
    <div class="stat"><span class="stat-label">Position</span><span id="pos" class="stat-value">-</span></div>
    <div class="stat"><span class="stat-label">Gold</span><span id="gold" class="stat-value gold">-</span></div>
    <div class="stat"><span class="stat-label">Shop Refreshes</span><span id="refreshes" class="stat-value">0</span></div>
    <div class="stat"><span class="stat-label">Claws Scanned</span><span id="scanned" class="stat-value">0</span></div>
    <div class="stat"><span class="stat-label">Claws Found</span><span id="found" class="stat-value match">0</span></div>

    <h2>Path Info</h2>
    <div class="stat"><span class="stat-label">Target</span><span id="target" class="stat-value">-</span></div>
    <div class="stat"><span class="stat-label">Steps</span><span id="steps" class="stat-value">-</span></div>
    <div class="stat"><span class="stat-label">Distance</span><span id="distance" class="stat-value">-</span></div>

    <h2>Markers</h2>
    <div id="markers" class="marker-list"></div>

    <h2>Log</h2>
    <div id="log" class="log"></div>
  </div>
  <div class="main">
    <div class="canvas-wrap">
      <canvas id="minimap"></canvas>
    </div>
  </div>
</div>
<script>
const $ = id => document.getElementById(id);
let prevMsg = '';

function update() {
  fetch('/api/state')
    .then(r => r.json())
    .then(s => {
      const statusEl = $('status');
      statusEl.textContent = s.status;
      statusEl.className = 'status ' + (s.status || '').split(' ')[0].toLowerCase();

      $('uptime').textContent = s.uptime || '-';
      $('area').textContent = s.playerArea || '-';
      $('pos').textContent = s.playerPos ? s.playerPos.X + ', ' + s.playerPos.Y : '-';
      $('gold').textContent = s.goldCurrent ? s.goldCurrent.toLocaleString() : '-';
      $('refreshes').textContent = s.shopRefreshes;
      $('scanned').textContent = s.clawsScanned;
      $('found').textContent = s.clawsFound;
      $('target').textContent = s.targetNpc || '-';
      $('steps').textContent = s.pathSteps || '-';
      $('distance').textContent = s.pathDistance || '-';

      // Markers
      let mhtml = '';
      (s.markers || []).forEach(m => {
        mhtml += '<div class="marker-item ' + m.type + '">' + m.label + ' (' + m.worldX + ',' + m.worldY + ')</div>';
      });
      $('markers').innerHTML = mhtml;

      // Log
      if (s.lastMessage && s.lastMessage !== prevMsg) {
        const logEl = $('log');
        const ts = new Date().toLocaleTimeString();
        logEl.innerHTML += '<div>[' + ts + '] ' + s.lastMessage + '</div>';
        logEl.scrollTop = logEl.scrollHeight;
        prevMsg = s.lastMessage;
      }

      // Draw minimap
      drawMinimap(s);
    })
    .catch(() => {
      $('status').textContent = 'Disconnected';
      $('status').className = 'status error';
    });
}

function drawMinimap(state) {
  const canvas = $('minimap');
  const ctx = canvas.getContext('2d');
  canvas.width = canvas.parentElement.clientWidth;
  canvas.height = canvas.parentElement.clientHeight;
  const w = canvas.width, h = canvas.height;
  const cx = w / 2, cy = h / 2;

  ctx.fillStyle = '#0a0a0a';
  ctx.fillRect(0, 0, w, h);

  // Grid
  ctx.strokeStyle = '#1a1a1a';
  ctx.lineWidth = 1;
  for (let i = 0; i < w; i += 40) { ctx.beginPath(); ctx.moveTo(i, 0); ctx.lineTo(i, h); ctx.stroke(); }
  for (let i = 0; i < h; i += 40) { ctx.beginPath(); ctx.moveTo(0, i); ctx.lineTo(w, i); ctx.stroke(); }

  if (!state.playerPos) return;
  const px = state.playerPos.X || 0;
  const py = state.playerPos.Y || 0;
  const scale = 4;

  // Draw markers
  (state.markers || []).forEach(m => {
    const sx = cx + (m.worldX - px) * scale;
    const sy = cy + (m.worldY - py) * scale;
    if (sx < -20 || sx > w + 20 || sy < -20 || sy > h + 20) return;

    ctx.fillStyle = m.color || '#fff';
    ctx.beginPath();
    if (m.type === 'waypoint') {
      ctx.rect(sx - m.size/2, sy - m.size/2, m.size, m.size);
    } else {
      ctx.arc(sx, sy, m.size/2, 0, Math.PI * 2);
    }
    ctx.fill();

    ctx.fillStyle = '#fff';
    ctx.font = '10px monospace';
    ctx.fillText(m.label, sx + m.size, sy - 2);
  });

  // Draw player
  ctx.fillStyle = '#ff4a4a';
  ctx.beginPath();
  ctx.arc(cx, cy, 6, 0, Math.PI * 2);
  ctx.fill();
  ctx.fillStyle = '#fff';
  ctx.font = '11px monospace';
  ctx.fillText('YOU', cx + 10, cy + 4);
}

setInterval(update, 200);
update();
</script>
</body>
</html>`
