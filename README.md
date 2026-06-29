# ONVIF Servo Proxy

Rock 3C service that exposes an ONVIF PTZ camera facade for Frigate while splitting PTZ commands across:

- pan/tilt: UART commands to the ESP32 MicroPython gimbal firmware in `firmware/`
- zoom: the real camera's optical zoom through HTTP hooks now, with an ONVIF-client backend reserved for the next pass
- media: `GetStreamUri` returns the real camera or MediaMTX RTSP URL

## Current Shape

The first framework is intentionally dependency-light so it cross-compiles cleanly for ARM64:

- `cmd/onvif-servo-proxy`: service entrypoint
- `internal/onvif`: small ONVIF Device/Media/PTZ SOAP facade plus WS-Discovery responder
- `internal/ptz`: composite PTZ controller
- `internal/servo`: UART client compatible with the current ESP32 commands
- `internal/camera`: optical zoom adapter interface and HTTP hook implementation
- `internal/web`: lightweight configuration UI and test API

The ESP32 firmware already accepts:

```text
center [s=90]
status
move <pan> <tilt> [s=90]
step <dpan> <dtilt> [s=90]
```

## Build

```bash
go build ./cmd/onvif-servo-proxy
```

For Rock 3C:

```bash
scripts/build-rock3c.sh
```

## Run Locally

```bash
go run ./cmd/onvif-servo-proxy -config ./configs/onvif-servo-proxy.example.json
```

Open:

```text
http://localhost:8080/
```

ONVIF device endpoint:

```text
http://localhost:8080/onvif/device_service
```

## Deploy To Rock 3C

The deploy script defaults to `radxa@192.168.1.128`:

```bash
scripts/deploy-rock3c.sh
```

Then run manually:

```bash
ssh radxa@192.168.1.128 '/home/radxa/Apps/onvif-servo-proxy -config /home/radxa/Apps/onvif-servo-proxy.json'
```

To install systemd service:

```bash
scp packaging/systemd/onvif-servo-proxy.service radxa@192.168.1.128:/tmp/
ssh radxa@192.168.1.128 'sudo mv /tmp/onvif-servo-proxy.service /etc/systemd/system/ && sudo systemctl daemon-reload && sudo systemctl enable --now onvif-servo-proxy'
```

## Frigate

Point Frigate ONVIF at:

```yaml
onvif:
  host: 192.168.1.128
  port: 8080
  user: admin
  password: admin
```

Use the web UI to set `camera.stream_uri` to the real camera RTSP or a MediaMTX/go2rtc restream URL.

## Next Hardware Bring-up Checks

1. Confirm the Rock 3C serial device name, for example `/dev/ttyS1` or `/dev/ttyUSB0`.
2. From the web UI, send `status`, then `center s=60`.
3. Set `server.public_host` to `192.168.1.128`.
4. Set `camera.stream_uri` to the real camera RTSP URL.
5. Add Frigate ONVIF config and inspect logs for missing PTZ capability names.

If Frigate requires stricter ONVIF behavior than this minimal facade provides, the next step is replacing `internal/onvif` with an onvif-go server fork while keeping the existing `internal/ptz`, `internal/servo`, `internal/camera`, and `internal/web` layers.
