package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"onvif-servo-proxy/internal/config"
	"onvif-servo-proxy/internal/servo"
	"onvif-servo-proxy/internal/version"
)

type Server struct {
	path  string
	cfg   config.Config
	servo *servo.Client
}

func NewServer(path string, cfg config.Config, servoClient *servo.Client) *Server {
	return &Server{path: path, cfg: cfg, servo: servoClient}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", s.index)
	mux.HandleFunc("/api/config", s.configAPI)
	mux.HandleFunc("/api/servo/status", s.servoStatus)
	mux.HandleFunc("/api/servo/raw", s.servoRaw)
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = page.Execute(w, pageData{Config: s.cfg, Version: version.Short()})
}

func (s *Server) configAPI(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.cfg)
	case http.MethodPost, http.MethodPut:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cfg.Normalize()
		if err := config.Save(s.path, cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.cfg = cfg
		if r.URL.Query().Get("restart") == "1" {
			writeJSON(w, map[string]string{"status": "saved", "note": "service restart scheduled"})
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			go func() {
				time.Sleep(350 * time.Millisecond)
				os.Exit(1)
			}()
			return
		}
		writeJSON(w, map[string]string{"status": "saved", "note": "configuration saved"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) servoStatus(w http.ResponseWriter, r *http.Request) {
	st, err := s.servo.Status(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, st)
}

func (s *Server) servoRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Command string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp, err := s.servo.Raw(r.Context(), strings.TrimSpace(req.Command))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, map[string]string{"response": resp})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, fmt.Sprintf("encode json: %v", err), http.StatusInternalServerError)
	}
}

type pageData struct {
	config.Config
	Version string
}

var page = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ONVIF Servo Proxy</title>
  <style>
    :root { color-scheme: light; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; --ink:#202124; --muted:#66736f; --line:#d7dedb; --panel:#ffffff; --soft:#eef4f1; --green:#176b5c; --green2:#0f4f45; --slate:#485762; --amber:#b56a00; }
    body { margin: 0; background: #f4f6f3; color: var(--ink); }
    header { background: #153243; color: white; padding: 18px 24px; box-shadow: 0 2px 10px rgba(13, 32, 43, .18); }
    h1 { margin: 0; font-size: 22px; letter-spacing: 0; }
    main { max-width: 1180px; margin: 0 auto; padding: 24px; display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 18px; }
    section { background: var(--panel); border: 1px solid var(--line); border-radius: 8px; padding: 18px; box-shadow: 0 1px 2px rgba(20, 32, 28, .04); }
    h2 { margin: 0 0 14px; font-size: 17px; }
    h3 { margin: 0 0 10px; font-size: 14px; color: #31413d; }
    label { display: block; font-size: 13px; color: #44514d; margin: 10px 0 4px; }
    input, select, textarea { width: 100%; box-sizing: border-box; border: 1px solid #b8c2be; border-radius: 6px; padding: 9px 10px; font: inherit; background: white; color: var(--ink); outline: none; }
    input:focus, textarea:focus { border-color: var(--green); box-shadow: 0 0 0 3px rgba(23, 107, 92, .12); }
    textarea { min-height: 300px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; line-height: 1.45; }
    button { position: relative; border: 0; border-radius: 6px; padding: 9px 13px; font: inherit; cursor: pointer; background: var(--green); color: white; box-shadow: 0 3px 0 var(--green2); transition: transform .08s ease, box-shadow .08s ease, filter .15s ease, background .15s ease; user-select: none; }
    button:hover { filter: brightness(1.06); }
    button:active, button.pressed { transform: translateY(2px) scale(.985); box-shadow: 0 1px 0 var(--green2); }
    button.secondary { background: var(--slate); box-shadow: 0 3px 0 #313d45; }
    button.warning { background: var(--amber); box-shadow: 0 3px 0 #744400; }
    button.busy { pointer-events: none; filter: saturate(.8); }
    button.busy::after { content: ""; width: 12px; height: 12px; margin-left: 8px; display: inline-block; vertical-align: -2px; border: 2px solid rgba(255,255,255,.45); border-top-color: white; border-radius: 50%; animation: spin .7s linear infinite; }
    @keyframes spin { to { transform: rotate(360deg); } }
    .row { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
    .actions { display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap; align-items: center; }
    .status { white-space: pre-wrap; background: var(--soft); border: 1px solid #dce5e1; border-radius: 6px; padding: 10px; min-height: 42px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; overflow-wrap: anywhere; }
    .span-all { grid-column: 1 / -1; }
    .visual-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; }
    .viz { border: 1px solid #d8e1dd; border-radius: 8px; padding: 12px; background: #fbfcfb; }
    .viz svg { width: 100%; height: 210px; display: block; }
    .raw-panel { margin-top: 14px; }
    .control-visuals { margin-top: 16px; }
    footer { max-width: 1180px; margin: 0 auto; padding: 0 24px 24px; color: var(--muted); font-size: 12px; text-align: center; }
    footer a { color: var(--green); text-decoration: none; }
    footer a:hover { text-decoration: underline; }
    .beam { transition: transform .25s ease; transform-origin: 110px 110px; }
    .tilt-beam { transition: transform .25s ease; transform-origin: 34px 110px; }
    @media (max-width: 860px) { main { grid-template-columns: 1fr; padding: 14px; } .row, .visual-grid { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
<header><h1>ONVIF Servo Proxy</h1></header>
<main>
  <section>
    <h2>基础配置</h2>
    <div class="row">
      <div><label>HTTP 端口</label><input id="port" type="number" value="{{.Server.Port}}"></div>
      <div><label>Public Host</label><input id="public_host" value="{{.Server.PublicHost}}" placeholder="rock3c-ip"></div>
    </div>
    <div class="row">
      <div><label>ONVIF 用户名</label><input id="username" value="{{.Server.Username}}"></div>
      <div><label>ONVIF 密码</label><input id="password" value="{{.Server.Password}}"></div>
    </div>
    <label>真实摄像头 RTSP URI</label><input id="stream_uri" value="{{.Camera.StreamURI}}">
    <label>Snapshot URI</label><input id="snapshot_uri" value="{{.Camera.SnapshotURI}}">
    <div class="row">
      <div><label>串口设备</label><input id="serial_device" value="{{.Servo.SerialDevice}}"></div>
      <div><label>串口波特率</label><input id="baud" type="number" value="{{.Servo.Baud}}"></div>
    </div>
    <div class="actions">
      <button onclick="saveSimple(event)">保存配置</button>
      <button class="warning" onclick="saveSimple(event, true)">保存并重启</button>
      <button class="secondary" onclick="loadConfig(event)">刷新 JSON</button>
    </div>
    <p class="status" id="save_status"></p>
  </section>

  <section>
    <h2>舵机控制</h2>
    <label>命令</label><input id="servo_command" value="status">
    <div class="actions">
      <button onclick="sendServo(event)">发送</button>
      <button class="secondary" onclick="quick(event, 'center s=90')">居中</button>
      <button class="secondary" onclick="quick(event, 'move 135 90 s=90')">Home</button>
      <button class="secondary" onclick="quick(event, 'status')">状态</button>
    </div>
    <div class="visual-grid control-visuals">
      <div class="viz">
        <h3>Pan 顶视图</h3>
        <svg viewBox="0 0 220 220" aria-label="pan status">
          <path d="M45 175 A92 92 0 1 1 175 175" fill="none" stroke="#d0dad6" stroke-width="16" stroke-linecap="round"/>
          <path d="M45 175 A92 92 0 1 1 175 175" fill="none" stroke="#9bb0aa" stroke-width="2" stroke-dasharray="4 5"/>
          <line x1="110" y1="110" x2="110" y2="28" stroke="#1d6f63" stroke-width="8" stroke-linecap="round" class="beam" id="pan_beam"/>
          <circle cx="110" cy="110" r="24" fill="#153243"/>
          <rect x="98" y="86" width="24" height="48" rx="7" fill="#244b5b"/>
          <circle cx="110" cy="110" r="7" fill="#dce7e3"/>
          <text x="36" y="196" font-size="11" fill="#66736f">min</text>
          <text x="168" y="196" font-size="11" fill="#66736f">max</text>
        </svg>
      </div>
      <div class="viz">
        <h3>Tilt 侧视图</h3>
        <svg viewBox="0 0 220 220" aria-label="tilt status">
          <line x1="34" y1="110" x2="196" y2="110" stroke="#d0dad6" stroke-width="2" stroke-dasharray="5 5"/>
          <path d="M34 185 A75 75 0 0 0 34 35" fill="none" stroke="#d0dad6" stroke-width="14" stroke-linecap="round"/>
          <line x1="34" y1="110" x2="160" y2="110" stroke="#1d6f63" stroke-width="8" stroke-linecap="round" class="tilt-beam" id="tilt_beam"/>
          <circle cx="34" cy="110" r="21" fill="#153243"/>
          <rect x="20" y="96" width="28" height="28" rx="6" fill="#244b5b"/>
          <text x="166" y="105" font-size="11" fill="#66736f">90 水平</text>
          <text x="22" y="27" font-size="11" fill="#66736f">max</text>
          <text x="22" y="204" font-size="11" fill="#66736f">min</text>
        </svg>
      </div>
    </div>
    <div class="raw-panel">
      <h3>报文</h3>
      <p class="status" id="servo_status">等待命令...</p>
    </div>
  </section>

  <section class="span-all">
    <h2>完整 JSON</h2>
    <textarea id="config_json"></textarea>
    <div class="actions">
      <button onclick="saveJSON(event)">保存 JSON</button>
      <button class="warning" onclick="saveJSON(event, true)">保存 JSON 并重启</button>
    </div>
  </section>
</main>
<footer>Made in ❤️ with <a href="https://everains.com/" target="_blank" rel="noopener noreferrer">Raiot</a> and Codex · Version: {{.Version}}</footer>
<script>
let cfg = null;
let lastStatus = null;

async function loadConfig(event) {
  return withButton(event, async () => {
    const res = await fetch('/api/config');
    cfg = await res.json();
    document.getElementById('config_json').value = JSON.stringify(cfg, null, 2);
    if (lastStatus) updateVisuals(lastStatus);
    return cfg;
  });
}

function syncSimple() {
  cfg.server.port = Number(document.getElementById('port').value);
  cfg.server.public_host = document.getElementById('public_host').value;
  cfg.server.username = document.getElementById('username').value;
  cfg.server.password = document.getElementById('password').value;
  cfg.camera.stream_uri = document.getElementById('stream_uri').value;
  cfg.camera.snapshot_uri = document.getElementById('snapshot_uri').value;
  cfg.servo.serial_device = document.getElementById('serial_device').value;
  cfg.servo.baud = Number(document.getElementById('baud').value);
}

async function saveSimple(event, restart = false) {
  syncSimple();
  await save(event, cfg, restart);
}

async function saveJSON(event, restart = false) {
  await save(event, JSON.parse(document.getElementById('config_json').value), restart);
}

async function save(event, next, restart) {
  return withButton(event, async () => {
    const url = restart ? '/api/config?restart=1' : '/api/config';
    const res = await fetch(url, {method:'PUT', headers:{'Content-Type':'application/json'}, body:JSON.stringify(next)});
    const text = await res.text();
    document.getElementById('save_status').textContent = text;
    if (restart) {
      document.getElementById('save_status').textContent += '\n服务正在重启，约 2 秒后恢复。';
      setTimeout(() => loadConfig().catch(() => {}), 2200);
      return;
    }
    await loadConfig();
  });
}

async function sendServo(event) {
  return withButton(event, async () => {
    const command = document.getElementById('servo_command').value;
    const res = await fetch('/api/servo/raw', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({command})});
    const text = await res.text();
    document.getElementById('servo_status').textContent = text;
    try {
      const payload = JSON.parse(text);
      updateFromResponse(payload.response || '');
    } catch (_) {
      updateFromResponse(text);
    }
  });
}

function quick(event, command) {
  document.getElementById('servo_command').value = command;
  return sendServo(event);
}

async function refreshStatus() {
  document.getElementById('servo_command').value = 'status';
  const res = await fetch('/api/servo/raw', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({command:'status'})});
  const payload = await res.json();
  document.getElementById('servo_status').textContent = JSON.stringify(payload);
  updateFromResponse(payload.response || '');
}

function withButton(event, fn) {
  const button = event && event.currentTarget;
  if (button) {
    button.classList.add('pressed', 'busy');
    setTimeout(() => button.classList.remove('pressed'), 140);
  }
  return Promise.resolve()
    .then(fn)
    .catch(err => {
      const target = document.getElementById('save_status') || document.getElementById('servo_status');
      target.textContent = String(err);
      throw err;
    })
    .finally(() => {
      if (button) button.classList.remove('busy');
    });
}

function updateFromResponse(text) {
  const parsed = parseStatus(text);
  if (!parsed) return;
  lastStatus = parsed;
  updateVisuals(parsed);
}

function parseStatus(text) {
  const match = /pan=([-0-9.]+)\/([-0-9.]+),\s*tilt=([-0-9.]+)\/([-0-9.]+)/.exec(text);
  if (!match) return null;
  return {
    pan: Number(match[1]),
    panTarget: Number(match[2]),
    tilt: Number(match[3]),
    tiltTarget: Number(match[4])
  };
}

function updateVisuals(status) {
  const s = cfg && cfg.servo ? cfg.servo : {pan_min:0, pan_max:270, tilt_min:0, tilt_max:180};
  const panNorm = clamp((status.pan - s.pan_min) / (s.pan_max - s.pan_min), 0, 1);
  const panDeg = -135 + panNorm * 270;
  const tiltDeg = 90 - status.tilt;
  document.getElementById('pan_beam').style.transform = 'rotate(' + panDeg + 'deg)';
  document.getElementById('tilt_beam').style.transform = 'rotate(' + tiltDeg + 'deg)';
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

loadConfig().then(() => refreshStatus().catch(() => {}));
</script>
</body>
</html>`))
