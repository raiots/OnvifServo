package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"onvif-servo-proxy/internal/camera"
	"onvif-servo-proxy/internal/config"
	"onvif-servo-proxy/internal/onvif"
	"onvif-servo-proxy/internal/ptz"
	"onvif-servo-proxy/internal/servo"
	"onvif-servo-proxy/internal/web"
)

func main() {
	var configPath string
	var writeDefault bool
	flag.StringVar(&configPath, "config", defaultConfigPath(), "path to JSON config")
	flag.BoolVar(&writeDefault, "write-default-config", false, "write a default config file and exit")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if writeDefault {
		cfg := config.Default()
		if err := config.Save(configPath, cfg); err != nil {
			logger.Error("write default config", "error", err)
			os.Exit(1)
		}
		logger.Info("default config written", "path", configPath)
		return
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("load config", "path", configPath, "error", err)
		os.Exit(1)
	}
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := config.Save(configPath, cfg); err != nil {
			logger.Warn("write initial config", "path", configPath, "error", err)
		}
	}

	servoClient := servo.NewClient(cfg.Servo)
	zoom := camera.NewController(cfg.Camera)
	ptzController := ptz.NewController(cfg.Servo, servoClient, zoom)

	mux := http.NewServeMux()
	onvif.NewServer(cfg, ptzController, logger).Register(mux)
	web.NewServer(configPath, cfg, servoClient).Register(mux)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go onvif.StartDiscovery(ctx, cfg, logger)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Bind, cfg.Server.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("onvif servo proxy listening", "addr", addr, "config", configPath)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Warn("http shutdown", "error", err)
	}
	_ = servoClient.Close()
}

func defaultConfigPath() string {
	if path := os.Getenv("ONVIF_SERVO_CONFIG"); path != "" {
		return path
	}
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "onvif-servo-proxy", "config.json")
	}
	return "config.json"
}
