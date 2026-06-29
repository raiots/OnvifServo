package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"onvif-servo-proxy/internal/config"
	"onvif-servo-proxy/internal/servo"
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
	_ = page.Execute(w, s.cfg)
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
		writeJSON(w, map[string]string{"status": "saved", "note": "restart the service to apply ONVIF endpoint changes"})
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

var page = template.Must(template.New("index").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>ONVIF Servo Proxy</title>
  <style>
    :root { color-scheme: light dark; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
    body { margin: 0; background: #f7f7f4; color: #202124; }
    header { background: #153243; color: white; padding: 18px 24px; }
    h1 { margin: 0; font-size: 22px; letter-spacing: 0; }
    main { max-width: 1120px; margin: 0 auto; padding: 24px; display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }
    section { background: white; border: 1px solid #d9dedb; border-radius: 8px; padding: 18px; }
    h2 { margin: 0 0 14px; font-size: 17px; }
    label { display: block; font-size: 13px; color: #445; margin: 10px 0 4px; }
    input, select, textarea { width: 100%; box-sizing: border-box; border: 1px solid #b9c1be; border-radius: 6px; padding: 9px 10px; font: inherit; background: white; color: #202124; }
    textarea { min-height: 260px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
    button { border: 0; border-radius: 6px; padding: 9px 12px; font: inherit; cursor: pointer; background: #1d6f63; color: white; }
    button.secondary { background: #4b5963; }
    .row { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
    .actions { display: flex; gap: 8px; margin-top: 12px; flex-wrap: wrap; }
    .status { white-space: pre-wrap; background: #eef2ef; border-radius: 6px; padding: 10px; min-height: 42px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
    @media (max-width: 820px) { main { grid-template-columns: 1fr; padding: 14px; } .row { grid-template-columns: 1fr; } }
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
      <button onclick="saveSimple()">保存配置</button>
      <button class="secondary" onclick="loadConfig()">刷新 JSON</button>
    </div>
    <p class="status" id="save_status"></p>
  </section>

  <section>
    <h2>舵机测试</h2>
    <label>命令</label><input id="servo_command" value="status">
    <div class="actions">
      <button onclick="sendServo()">发送</button>
      <button class="secondary" onclick="quick('center s=90')">居中</button>
      <button class="secondary" onclick="quick('move 135 90 s=90')">Home</button>
      <button class="secondary" onclick="quick('status')">状态</button>
    </div>
    <p class="status" id="servo_status"></p>
  </section>

  <section style="grid-column: 1 / -1">
    <h2>完整 JSON</h2>
    <textarea id="config_json"></textarea>
    <div class="actions"><button onclick="saveJSON()">保存 JSON</button></div>
  </section>
</main>
<script>
let cfg = null;
async function loadConfig() {
  const res = await fetch('/api/config');
  cfg = await res.json();
  document.getElementById('config_json').value = JSON.stringify(cfg, null, 2);
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
async function saveSimple() { syncSimple(); await save(cfg); }
async function saveJSON() { await save(JSON.parse(document.getElementById('config_json').value)); }
async function save(next) {
  const res = await fetch('/api/config', {method:'PUT', headers:{'Content-Type':'application/json'}, body:JSON.stringify(next)});
  document.getElementById('save_status').textContent = await res.text();
  await loadConfig();
}
async function sendServo() {
  const command = document.getElementById('servo_command').value;
  const res = await fetch('/api/servo/raw', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({command})});
  document.getElementById('servo_status').textContent = await res.text();
}
function quick(command) { document.getElementById('servo_command').value = command; sendServo(); }
loadConfig();
</script>
</body>
</html>`))
