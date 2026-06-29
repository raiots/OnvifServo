# ONVIF Servo Proxy

中文 | [English](README.md)

这是一个运行在 Rock 3C 上的服务，用于给 Frigate 暴露一个 ONVIF PTZ 摄像机外观，同时把 PTZ 命令拆分到不同后端：

- pan/tilt：通过 UART 发送命令到 `firmware/` 中的 ESP32 MicroPython 云台固件
- zoom：当前通过 HTTP hook 控制真实摄像机的光学变焦，下一轮预留 ONVIF 客户端后端
- media：`GetStreamUri` 返回真实摄像机或 MediaMTX 的 RTSP URL

## 当前结构

第一版框架刻意保持轻依赖，方便干净地交叉编译到 ARM64：

- `cmd/onvif-servo-proxy`：服务入口
- `internal/onvif`：小型 ONVIF Device/Media/PTZ SOAP 外观，以及 WS-Discovery 响应器
- `internal/ptz`：组合式 PTZ 控制器
- `internal/servo`：兼容当前 ESP32 命令的 UART 客户端
- `internal/camera`：光学变焦适配器接口和 HTTP hook 实现
- `internal/web`：轻量配置 UI 和测试 API

ESP32 固件目前接受这些命令：

```text
center [s=90]
status
move <pan> <tilt> [s=90]
step <dpan> <dtilt> [s=90]
```

## 构建

```bash
go build ./cmd/onvif-servo-proxy
```

面向 Rock 3C 构建：

```bash
scripts/build-rock3c.sh
```

## 本地运行

```bash
go run ./cmd/onvif-servo-proxy -config ./configs/onvif-servo-proxy.example.json
```

打开：

```text
http://localhost:8080/
```

ONVIF 设备端点：

```text
http://localhost:8080/onvif/device_service
```

## 部署到 Rock 3C

部署脚本默认目标是 `radxa@192.168.1.128`：

```bash
scripts/deploy-rock3c.sh
```

然后可以手动运行：

```bash
ssh radxa@192.168.1.128 '/home/radxa/Apps/onvif-servo-proxy -config /home/radxa/Apps/onvif-servo-proxy.json'
```

安装 systemd 服务：

```bash
scp packaging/systemd/onvif-servo-proxy.service radxa@192.168.1.128:/tmp/
ssh radxa@192.168.1.128 'sudo mv /tmp/onvif-servo-proxy.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable --now onvif-servo-proxy'
```

## Frigate

将 Frigate 的 ONVIF 指向：

```yaml
onvif:
  host: 192.168.1.128
  port: 8080
  user: admin
  password: admin
```

使用 Web UI 将 `camera.stream_uri` 设置为真实摄像机的 RTSP URL，或者 MediaMTX/go2rtc 的转流 URL。

## 下一步硬件联调检查

1. 确认 Rock 3C 上的串口设备名，例如 `/dev/ttyS1` 或 `/dev/ttyUSB0`。
2. 在 Web UI 中发送 `status`，然后发送 `center s=60`。
3. 将 `server.public_host` 设置为 `192.168.1.128`。
4. 将 `camera.stream_uri` 设置为真实摄像机的 RTSP URL。
5. 添加 Frigate ONVIF 配置，并检查日志里是否缺少 PTZ capability 名称。

如果 Frigate 要求比这个最小外观更严格的 ONVIF 行为，下一步是在保留现有 `internal/ptz`、`internal/servo`、`internal/camera` 和 `internal/web` 分层的同时，用 onvif-go server fork 替换 `internal/onvif`。
