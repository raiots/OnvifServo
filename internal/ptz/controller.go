package ptz

import (
	"context"
	"sync"
	"time"

	"onvif-servo-proxy/internal/camera"
	"onvif-servo-proxy/internal/config"
	"onvif-servo-proxy/internal/servo"
)

type Controller struct {
	cfg   config.ServoConfig
	servo *servo.Client
	zoom  camera.ZoomController

	mu     sync.Mutex
	status Status
}

func NewController(cfg config.ServoConfig, servoClient *servo.Client, zoom camera.ZoomController) *Controller {
	return &Controller{
		cfg:   cfg,
		servo: servoClient,
		zoom:  zoom,
		status: Status{
			Pan:       cfg.PanCenter,
			Tilt:      cfg.TiltCenter,
			UpdatedAt: time.Now(),
		},
	}
}

func (c *Controller) ContinuousMove(req MoveRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CommandTimeout())
	defer cancel()
	var firstErr error
	if req.PanTilt != nil {
		dpan := req.PanTilt.X * c.cfg.VelocityPanDPS * timeoutSeconds(req.Timeout)
		dtilt := req.PanTilt.Y * c.cfg.VelocityTiltDPS * timeoutSeconds(req.Timeout)
		speed := c.speed(req.Speed)
		st, err := c.servo.MoveBy(ctx, dpan, dtilt, speed)
		if err != nil {
			firstErr = err
		} else {
			c.updateServoStatus(st, req.PanTilt.X != 0, req.PanTilt.Y != 0)
		}
	}
	if req.Zoom != nil && c.zoom != nil {
		if err := c.zoom.Continuous(ctx, req.Zoom.X); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Controller) RelativeMove(req MoveRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CommandTimeout())
	defer cancel()
	var firstErr error
	if req.PanTilt != nil {
		dpan := req.PanTilt.X * c.cfg.RelativePanDegreesPerUnit
		dtilt := req.PanTilt.Y * c.cfg.RelativeTiltDegreesPerUnit
		st, err := c.servo.MoveBy(ctx, dpan, dtilt, c.speed(req.Speed))
		if err != nil {
			firstErr = err
		} else {
			c.updateServoStatus(st, dpan != 0, dtilt != 0)
		}
	}
	if req.Zoom != nil && c.zoom != nil {
		if err := c.zoom.Relative(ctx, req.Zoom.X); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Controller) AbsoluteMove(req MoveRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CommandTimeout())
	defer cancel()
	var firstErr error
	if req.PanTilt != nil {
		pan := normalizedToRange(req.PanTilt.X, c.cfg.PanMin, c.cfg.PanMax)
		tilt := normalizedToRange(req.PanTilt.Y, c.cfg.TiltMin, c.cfg.TiltMax)
		st, err := c.servo.MoveTo(ctx, pan, tilt, c.speed(req.Speed))
		if err != nil {
			firstErr = err
		} else {
			c.updateServoStatus(st, true, true)
		}
	}
	if req.Zoom != nil && c.zoom != nil {
		if err := c.zoom.Absolute(ctx, req.Zoom.X); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Controller) Stop(panTilt bool, zoomAxis bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CommandTimeout())
	defer cancel()
	var firstErr error
	if panTilt || !zoomAxis {
		st, err := c.servo.Status(ctx)
		if err != nil {
			firstErr = err
		} else {
			c.updateServoStatus(st, false, false)
		}
	}
	if (zoomAxis || !panTilt) && c.zoom != nil {
		if err := c.zoom.Stop(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (c *Controller) Home() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CommandTimeout())
	defer cancel()
	st, err := c.servo.Center(ctx, c.cfg.DefaultSpeedDPS)
	if err != nil {
		return err
	}
	c.updateServoStatus(st, true, true)
	return nil
}

func (c *Controller) Status() (Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.cfg.CommandTimeout())
	defer cancel()
	st, err := c.servo.Status(ctx)
	if err == nil {
		c.updateServoStatus(st, false, false)
	}
	if c.zoom != nil {
		zoom, moving, zerr := c.zoom.Status(ctx)
		c.mu.Lock()
		c.status.Zoom = zoom
		c.status.ZoomMoving = moving
		c.status.UpdatedAt = time.Now()
		current := c.status
		c.mu.Unlock()
		if err != nil {
			return current, err
		}
		return current, zerr
	}
	c.mu.Lock()
	current := c.status
	c.mu.Unlock()
	return current, err
}

func (c *Controller) updateServoStatus(st servo.Status, panMoving, tiltMoving bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.Pan = st.Pan
	c.status.Tilt = st.Tilt
	c.status.PanMoving = panMoving || abs(st.Pan-st.PanTarget) > 0.1
	c.status.TiltMoving = tiltMoving || abs(st.Tilt-st.TiltTarget) > 0.1
	c.status.UpdatedAt = time.Now()
}

func (c *Controller) speed(speed *Vector2D) float64 {
	if speed != nil {
		if speed.X != 0 {
			return abs(speed.X) * c.cfg.DefaultSpeedDPS
		}
		if speed.Y != 0 {
			return abs(speed.Y) * c.cfg.DefaultSpeedDPS
		}
	}
	return c.cfg.DefaultSpeedDPS
}

func normalizedToRange(v, min, max float64) float64 {
	if v < -1 {
		v = -1
	}
	if v > 1 {
		v = 1
	}
	return min + ((v + 1) / 2 * (max - min))
}

func timeoutSeconds(d time.Duration) float64 {
	if d <= 0 {
		return 0.2
	}
	seconds := d.Seconds()
	if seconds < 0.05 {
		return 0.05
	}
	if seconds > 1 {
		return 1
	}
	return seconds
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
