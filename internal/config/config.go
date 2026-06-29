package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Config struct {
	Server ServerConfig `json:"server"`
	Servo  ServoConfig  `json:"servo"`
	Camera CameraConfig `json:"camera"`
}

type ServerConfig struct {
	Bind       string `json:"bind"`
	Port       int    `json:"port"`
	PublicHost string `json:"public_host"`
	BasePath   string `json:"base_path"`
	Username   string `json:"username"`
	Password   string `json:"password"`

	Manufacturer    string `json:"manufacturer"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmware_version"`
	SerialNumber    string `json:"serial_number"`
	HardwareID      string `json:"hardware_id"`
}

type ServoConfig struct {
	SerialDevice      string  `json:"serial_device"`
	Baud              int     `json:"baud"`
	ConfigureWithStty bool    `json:"configure_with_stty"`
	PanMin            float64 `json:"pan_min"`
	PanMax            float64 `json:"pan_max"`
	PanCenter         float64 `json:"pan_center"`
	TiltMin           float64 `json:"tilt_min"`
	TiltMax           float64 `json:"tilt_max"`
	TiltCenter        float64 `json:"tilt_center"`
	DefaultSpeedDPS   float64 `json:"default_speed_dps"`
	CommandTimeoutMS  int     `json:"command_timeout_ms"`

	RelativePanDegreesPerUnit  float64 `json:"relative_pan_degrees_per_unit"`
	RelativeTiltDegreesPerUnit float64 `json:"relative_tilt_degrees_per_unit"`
	VelocityPanDPS             float64 `json:"velocity_pan_dps"`
	VelocityTiltDPS            float64 `json:"velocity_tilt_dps"`
}

type CameraConfig struct {
	StreamURI   string `json:"stream_uri"`
	SnapshotURI string `json:"snapshot_uri"`

	ZoomMode           string  `json:"zoom_mode"`
	ONVIFEndpoint      string  `json:"onvif_endpoint"`
	ONVIFUsername      string  `json:"onvif_username"`
	ONVIFPassword      string  `json:"onvif_password"`
	HTTPZoomInURL      string  `json:"http_zoom_in_url"`
	HTTPZoomOutURL     string  `json:"http_zoom_out_url"`
	HTTPZoomStopURL    string  `json:"http_zoom_stop_url"`
	HTTPZoomSetURL     string  `json:"http_zoom_set_url"`
	HTTPAuthHeader     string  `json:"http_auth_header"`
	HikvisionChannel   int     `json:"hikvision_channel"`
	HikvisionZoomSpeed int     `json:"hikvision_zoom_speed"`
	ZoomMin            float64 `json:"zoom_min"`
	ZoomMax            float64 `json:"zoom_max"`
	ZoomDefault        float64 `json:"zoom_default"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Bind:            "0.0.0.0",
			Port:            8080,
			BasePath:        "/onvif",
			Username:        "admin",
			Password:        "admin",
			Manufacturer:    "OnvifServo",
			Model:           "Rock 3C Servo PTZ Proxy",
			FirmwareVersion: "0.1.0",
			SerialNumber:    "ONVIF-SERVO-001",
			HardwareID:      "rock3c-esp32-gimbal",
		},
		Servo: ServoConfig{
			SerialDevice:               "/dev/ttyS2",
			Baud:                       115200,
			ConfigureWithStty:          true,
			PanMin:                     0,
			PanMax:                     270,
			PanCenter:                  135,
			TiltMin:                    0,
			TiltMax:                    180,
			TiltCenter:                 90,
			DefaultSpeedDPS:            90,
			CommandTimeoutMS:           900,
			RelativePanDegreesPerUnit:  30,
			RelativeTiltDegreesPerUnit: 20,
			VelocityPanDPS:             90,
			VelocityTiltDPS:            60,
		},
		Camera: CameraConfig{
			StreamURI:          "rtsp://camera-ip/stream1",
			ZoomMode:           "disabled",
			HikvisionChannel:   1,
			HikvisionZoomSpeed: 60,
			ZoomMin:            0,
			ZoomMax:            1,
			ZoomDefault:        0,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.Normalize()
	return cfg, nil
}

func Save(path string, cfg Config) error {
	cfg.Normalize()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func (c *Config) Normalize() {
	if c.Server.Bind == "" {
		c.Server.Bind = "0.0.0.0"
	}
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.BasePath == "" {
		c.Server.BasePath = "/onvif"
	}
	if c.Servo.Baud == 0 {
		c.Servo.Baud = 115200
	}
	if c.Servo.CommandTimeoutMS == 0 {
		c.Servo.CommandTimeoutMS = 900
	}
	if c.Servo.DefaultSpeedDPS == 0 {
		c.Servo.DefaultSpeedDPS = 90
	}
	if c.Camera.ZoomMode == "" {
		c.Camera.ZoomMode = "disabled"
	}
	if c.Camera.HikvisionChannel == 0 {
		c.Camera.HikvisionChannel = 1
	}
	if c.Camera.HikvisionZoomSpeed == 0 {
		c.Camera.HikvisionZoomSpeed = 60
	}
	if c.Camera.ZoomMax == c.Camera.ZoomMin {
		c.Camera.ZoomMax = 1
	}
}

func (s ServoConfig) CommandTimeout() time.Duration {
	return time.Duration(s.CommandTimeoutMS) * time.Millisecond
}
