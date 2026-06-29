package ptz

import "time"

type Vector2D struct {
	X float64
	Y float64
}

type Vector1D struct {
	X float64
}

type MoveRequest struct {
	ProfileToken string
	PanTilt      *Vector2D
	Zoom         *Vector1D
	Speed        *Vector2D
	Timeout      time.Duration
}

type Status struct {
	Pan        float64
	Tilt       float64
	Zoom       float64
	PanMoving  bool
	TiltMoving bool
	ZoomMoving bool
	UpdatedAt  time.Time
}

type Backend interface {
	ContinuousMove(req MoveRequest) error
	RelativeMove(req MoveRequest) error
	AbsoluteMove(req MoveRequest) error
	Stop(panTilt bool, zoom bool) error
	Status() (Status, error)
	Home() error
}
