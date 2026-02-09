package shopbot

const guiHTML = `<!DOCTYPE html>
<html>
<head>
<title>D2R Claw Shop Bot</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { background: #0d1117; color: #c9d1d9; font-family: 'Segoe UI', Consolas, monospace; font-size: 13px; }
.app { display: flex; height: 100vh; }
.sidebar { width: 360px; background: #161b22; border-right: 1px solid #30363d; display: flex; flex-direction: column; }
.main { flex: 1; display: flex; flex-direction: column; }
.panel { padding: 12px; border-bottom: 1px solid #30363d; }
h1 { color: #d4a74a; font-size: 18px; margin-bottom: 4px; }
h1 small { color: #666; font-size: 11px; font-weight: normal; }
h2 { color: #8b949e; font-size: 11px; text-transform: uppercase; letter-spacing: 1px; margin-bottom: 8px; }
.status-bar { padding: 8px 12px; font-weight: bold; text-align: center; border-radius: 4px; margin: 4px 0; }
.status-idle { background: #1c2128; color: #8b949e; border: 1px solid #30363d; }
.status-testing { background: #1a2332; color: #58a6ff; border: 1px solid #1f6feb; }
.status-shopping { background: #1a2b1a; color: #3fb950; border: 1px solid #238636; }
.status-error { background: #2d1b1b; color: #f85149; border: 1px solid #da3633; }
.status-break { background: #2d2a1b; color: #d29922; border: 1px solid #9e6a03; }
button { padding: 8px 16px; border: 1px solid #30363d; border-radius: 4px; cursor: pointer; font-size: 12px; font-family: inherit; transition: all 0.2s; }
button:hover { opacity: 0.85; }
.btn-primary { background: #238636; color: #fff; border-color: #2ea043; }
.btn-warning { background: #9e6a03; color: #fff; border-color: #d29922; }
.btn-danger { background: #da3633; color: #fff; border-color: #f85149; }
.btn-info { background: #1f6feb; color: #fff; border-color: #58a6ff; }
.btn-default { background: #21262d; color: #c9d1d9; }
button:disabled { opacity: 0.4; cursor: not-allowed; }
select { padding: 6px 8px; background: #0d1117; color: #c9d1d9; border: 1px solid #30363d; border-radius: 4px; width: 100%; font-size: 12px; }
input[type=number] { padding: 6px 8px; background: #0d1117; color: #c9d1d9; border: 1px solid #30363d; border-radius: 4px; width: 80px; font-size: 12px; }
input[type=checkbox] { margin-right: 6px; }
label { display: flex; align-items: center; margin: 4px 0; font-size: 12px; cursor: pointer; }
.btn-row { display: flex; gap: 6px; margin-top: 8px; flex-wrap: wrap; }
.log-container { flex: 1; overflow: hidden; display: flex; flex-direction: column; }
.log-header { padding: 8px 12px; background: #161b22; border-bottom: 1px solid #30363d; display: flex; justify-content: space-between; align-items: center; }
.log-body { flex: 1; overflow-y: auto; padding: 8px; font-family: Consolas, 'Courier New', monospace; font-size: 12px; background: #0d1117; }
.log-entry { padding: 2px 0; border-bottom: 1px solid #161b22; }
.log-time { color: #484f58; }
.log-info { color: #58a6ff; }
.log-warn { color: #d29922; }
.log-error { color: #f85149; }
.log-success { color: #3fb950; }
.game-state { display: grid; grid-template-columns: 1fr 1fr; gap: 4px; }
.gs-item { display: flex; justify-content: space-between; padding: 2px 4px; background: #161b22; border-radius: 2px; }
.gs-label { color: #8b949e; font-size: 11px; }
.gs-value { color: #fff; font-size: 11px; font-weight: bold; }
.gs-value.good { color: #3fb950; }
.gs-value.bad { color: #f85149; }
.gs-value.warn { color: #d29922; }
.stats-grid { display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 8px; padding: 12px; }
.stat-card { background: #161b22; border: 1px solid #30363d; border-radius: 6px; padding: 12px; text-align: center; }
.stat-value { font-size: 24px; font-weight: bold; color: #d4a74a; }
.stat-label { font-size: 10px; color: #8b949e; text-transform: uppercase; letter-spacing: 1px; margin-top: 4px; }
.overlay-section { padding: 12px; background: #161b22; }
.help-text { font-size: 11px; color: #484f58; margin-top: 4px; font-style: italic; }
.break-config { display: flex; flex-direction: column; gap: 6px; }
</style>
</head>
<body>
<div class="app">
  <div class="sidebar">
    <!-- Header -->
    <div class="panel">
      <h1>D2R CLAW SHOP BOT <small>v1.0</small></h1>
      <div id="statusBar" class="status-bar status-idle">Ready</div>
    </div>

    <!-- Process Detection -->
    <div class="panel">
      <h2>1. Detect D2R Clients</h2>
      <div class="btn-row">
        <button class="btn-info" onclick="detectProcesses()">Detect D2R Windows</button>
      </div>
      <select id="processSelect" style="margin-top:8px" disabled>
        <option value="">-- Click Detect first --</option>
      </select>
      <div class="btn-row">
        <button class="btn-primary" id="attachBtn" onclick="attachProcess()" disabled>Attach & Activate</button>
      </div>
    </div>

    <!-- Testing -->
    <div class="panel">
      <h2>2. Test & Verify</h2>
      <div class="btn-row">
        <button class="btn-info" id="testBtn" onclick="runTest()" disabled>Run Diagnostic Test</button>
        <button class="btn-warning" id="configBtn" onclick="configureInGame()" disabled>Configure In-Game</button>
      </div>
      <div class="help-text">Tests: Detect town, Anya, waypoint, red portal, shop window, quest status</div>
    </div>

    <!-- Game State -->
    <div class="panel">
      <h2>Game State</h2>
      <div id="gameState" class="game-state">
        <div class="gs-item"><span class="gs-label">Status</span><span class="gs-value" id="gsInGame">--</span></div>
        <div class="gs-item"><span class="gs-label">Area</span><span class="gs-value" id="gsArea">--</span></div>
        <div class="gs-item"><span class="gs-label">Position</span><span class="gs-value" id="gsPos">--</span></div>
        <div class="gs-item"><span class="gs-label">Gold</span><span class="gs-value" id="gsGold">--</span></div>
        <div class="gs-item"><span class="gs-label">Anya</span><span class="gs-value" id="gsAnya">--</span></div>
        <div class="gs-item"><span class="gs-label">Waypoint</span><span class="gs-value" id="gsWP">--</span></div>
        <div class="gs-item"><span class="gs-label">Red Portal</span><span class="gs-value" id="gsPortal">--</span></div>
        <div class="gs-item"><span class="gs-label">Legacy GFX</span><span class="gs-value" id="gsLegacy">--</span></div>
      </div>
    </div>

    <!-- Shopping Controls -->
    <div class="panel">
      <h2>3. Shopping</h2>
      <div class="btn-row">
        <button class="btn-primary" id="startBtn" onclick="startShopping()" disabled>Start Shopping</button>
        <button class="btn-danger" id="stopBtn" onclick="stopShopping()" disabled>Stop</button>
      </div>
    </div>

    <!-- Break Config -->
    <div class="panel">
      <h2>Break Settings</h2>
      <div class="break-config">
        <label><input type="checkbox" id="breakEnabled"> Enable random breaks</label>
        <div style="display:flex;gap:8px;align-items:center">
          <span style="font-size:11px">Break every</span>
          <input type="number" id="breakMinutes" value="30" min="1" max="999">
          <span style="font-size:11px">minutes</span>
        </div>
        <label><input type="checkbox" id="breakRandomize"> Randomize timing</label>
        <div style="display:flex;gap:8px;align-items:center">
          <span style="font-size:11px">Random +/-</span>
          <input type="number" id="breakRandomRange" value="5" min="0" max="60">
          <span style="font-size:11px">minutes</span>
        </div>
        <div class="help-text">When enabled, the bot will pause for the configured time. With randomize on, it adds or subtracts a random number of minutes within your range to look more natural.</div>
        <button class="btn-default" onclick="saveBreakSettings()">Save Break Settings</button>
      </div>
    </div>
  </div>

  <div class="main">
    <!-- Stats Bar -->
    <div class="stats-grid">
      <div class="stat-card"><div class="stat-value" id="statRefreshes">0</div><div class="stat-label">Refreshes</div></div>
      <div class="stat-card"><div class="stat-value" id="statScanned">0</div><div class="stat-label">Claws Scanned</div></div>
      <div class="stat-card"><div class="stat-value" id="statMatched">0</div><div class="stat-label">Matches</div></div>
      <div class="stat-card"><div class="stat-value" id="statPurchased">0</div><div class="stat-label">Purchased</div></div>
      <div class="stat-card"><div class="stat-value" id="statGold">0</div><div class="stat-label">Gold Spent</div></div>
      <div class="stat-card"><div class="stat-value" id="statPhase">IDLE</div><div class="stat-label">Phase</div></div>
    </div>

    <!-- Log Window -->
    <div class="log-container">
      <div class="log-header">
        <h2 style="margin:0">Activity Log</h2>
        <button class="btn-default" style="padding:4px 8px;font-size:11px" onclick="clearLog()">Clear</button>
      </div>
      <div class="log-body" id="logBody"></div>
    </div>
  </div>
</div>

<script>
const $ = id => document.getElementById(id);
let lastLogCount = 0;

function api(endpoint, method, body) {
  const opts = { method: method || 'GET' };
  if (body) {
    opts.headers = { 'Content-Type': 'application/json' };
    opts.body = JSON.stringify(body);
  }
  return fetch('/api/' + endpoint, opts).then(r => r.json()).catch(e => console.error(e));
}

function detectProcesses() {
  api('detect').then(procs => {
    const sel = $('processSelect');
    sel.innerHTML = '';
    if (!procs || procs.length === 0) {
      sel.innerHTML = '<option value="">No D2R processes found</option>';
      sel.disabled = true;
      $('attachBtn').disabled = true;
      return;
    }
    procs.forEach(p => {
      const opt = document.createElement('option');
      opt.value = p.pid;
      const char = p.characterName || '(unknown)';
      opt.textContent = 'PID ' + p.pid + ' - ' + p.windowTitle + ' [' + char + ']';
      sel.appendChild(opt);
    });
    sel.disabled = false;
    $('attachBtn').disabled = false;
  });
}

function attachProcess() {
  const pid = parseInt($('processSelect').value);
  if (!pid) return;
  $('attachBtn').disabled = true;
  $('attachBtn').textContent = 'Attaching...';
  api('attach', 'POST', { pid }).then(() => {
    $('testBtn').disabled = false;
    $('configBtn').disabled = false;
    $('startBtn').disabled = false;
    $('attachBtn').textContent = 'Attached!';
  }).catch(() => {
    $('attachBtn').disabled = false;
    $('attachBtn').textContent = 'Attach & Activate';
  });
}

function runTest() {
  $('testBtn').disabled = true;
  $('testBtn').textContent = 'Testing...';
  api('test').then(info => {
    updateGameState(info);
    $('testBtn').disabled = false;
    $('testBtn').textContent = 'Run Diagnostic Test';
  });
}

function configureInGame() {
  $('configBtn').disabled = true;
  $('configBtn').textContent = 'Configuring...';
  api('configure-ingame', 'POST').then(() => {
    setTimeout(() => {
      $('configBtn').disabled = false;
      $('configBtn').textContent = 'Configure In-Game';
    }, 5000);
  });
}

function startShopping() {
  api('start', 'POST').then(() => {
    $('startBtn').disabled = true;
    $('stopBtn').disabled = false;
  });
}

function stopShopping() {
  api('stop', 'POST').then(() => {
    $('startBtn').disabled = false;
    $('stopBtn').disabled = true;
  });
}

function saveBreakSettings() {
  api('save-break', 'POST', {
    enabled: $('breakEnabled').checked,
    minutes: parseInt($('breakMinutes').value) || 30,
    randomize: $('breakRandomize').checked,
    randomRange: parseInt($('breakRandomRange').value) || 5
  });
}

function clearLog() {
  $('logBody').innerHTML = '';
  lastLogCount = 0;
}

function updateGameState(info) {
  if (!info) return;
  $('gsInGame').textContent = info.inGame ? 'In Game' : 'Not In Game';
  $('gsInGame').className = 'gs-value ' + (info.inGame ? 'good' : 'bad');
  $('gsArea').textContent = info.area || '--';
  $('gsPos').textContent = info.playerX ? info.playerX + ',' + info.playerY : '--';
  $('gsGold').textContent = info.playerGold ? info.playerGold.toLocaleString() : '--';
  $('gsAnya').textContent = info.anyaVisible ? 'Visible (d=' + info.anyaDistance + ')' : 'Not found';
  $('gsAnya').className = 'gs-value ' + (info.anyaVisible ? 'good' : 'warn');
  $('gsWP').textContent = info.waypointNearby ? 'Found' : 'Not found';
  $('gsWP').className = 'gs-value ' + (info.waypointNearby ? 'good' : '');
  $('gsPortal').textContent = info.redPortalFound ? 'Found' : 'Not found';
  $('gsPortal').className = 'gs-value ' + (info.redPortalFound ? 'good' : '');
  $('gsLegacy').textContent = info.legacyGraphics ? 'ON' : 'OFF';
  $('gsLegacy').className = 'gs-value ' + (info.legacyGraphics ? 'good' : 'warn');
}

function updateState() {
  fetch('/api/state')
    .then(r => r.json())
    .then(s => {
      // Update status bar
      const bar = $('statusBar');
      bar.textContent = s.botStatus || 'Ready';
      bar.className = 'status-bar status-' + (s.botPhase || 'idle');

      // Update stats
      $('statRefreshes').textContent = s.shopRefreshes || 0;
      $('statScanned').textContent = s.clawsScanned || 0;
      $('statMatched').textContent = s.clawsMatched || 0;
      $('statPurchased').textContent = s.clawsPurchased || 0;
      $('statGold').textContent = (s.goldSpent || 0).toLocaleString();
      $('statPhase').textContent = (s.botPhase || 'idle').toUpperCase();

      // Update game state
      if (s.gameState) updateGameState(s.gameState);

      // Update log
      if (s.logMessages && s.logMessages.length > lastLogCount) {
        const logBody = $('logBody');
        const newEntries = s.logMessages.slice(lastLogCount);
        newEntries.forEach(entry => {
          const div = document.createElement('div');
          div.className = 'log-entry';
          const levelClass = entry.level === 'ERROR' ? 'log-error' :
                            entry.level === 'WARN' ? 'log-warn' :
                            entry.level === 'INFO' && entry.message.includes('GOOD') ? 'log-success' :
                            'log-info';
          div.innerHTML = '<span class="log-time">[' + entry.time + ']</span> ' +
                         '<span class="' + levelClass + '">[' + entry.level + ']</span> ' +
                         entry.message;
          logBody.appendChild(div);
        });
        logBody.scrollTop = logBody.scrollHeight;
        lastLogCount = s.logMessages.length;
      }

      // Update button states
      if (s.botRunning) {
        $('startBtn').disabled = true;
        $('stopBtn').disabled = false;
      }
    })
    .catch(() => {});
}

setInterval(updateState, 300);
updateState();
</script>
</body>
</html>`
