package servo

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"onvif-servo-proxy/internal/config"
)

type Status struct {
	Pan        float64
	PanTarget  float64
	Tilt       float64
	TiltTarget float64
	Raw        string
}

type Client struct {
	cfg config.ServoConfig
	mu  sync.Mutex
	rw  *os.File
	rd  *bufio.Reader
}

var statusPattern = regexp.MustCompile(`pan=([-0-9.]+)/([-0-9.]+),\s*tilt=([-0-9.]+)/([-0-9.]+)`)

func NewClient(cfg config.ServoConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Open() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.rw != nil {
		return nil
	}
	if c.cfg.SerialDevice == "" {
		return errors.New("servo serial_device is empty")
	}
	if c.cfg.ConfigureWithStty {
		if err := configureSerial(c.cfg.SerialDevice, c.cfg.Baud); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(c.cfg.SerialDevice, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return err
	}
	c.rw = f
	c.rd = bufio.NewReader(f)
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rw == nil {
		return nil
	}
	err := c.rw.Close()
	c.rw = nil
	c.rd = nil
	return err
}

func (c *Client) Center(ctx context.Context, speed float64) (Status, error) {
	return c.commandStatus(ctx, fmt.Sprintf("center s=%.1f", speed))
}

func (c *Client) MoveTo(ctx context.Context, pan, tilt, speed float64) (Status, error) {
	return c.commandStatus(ctx, fmt.Sprintf("move %.2f %.2f s=%.1f", pan, tilt, speed))
}

func (c *Client) MoveBy(ctx context.Context, dpan, dtilt, speed float64) (Status, error) {
	return c.commandStatus(ctx, fmt.Sprintf("step %.2f %.2f s=%.1f", dpan, dtilt, speed))
}

func (c *Client) Status(ctx context.Context) (Status, error) {
	return c.commandStatus(ctx, "status")
}

func (c *Client) Raw(ctx context.Context, line string) (string, error) {
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.CommandTimeout())
		defer cancel()
	}
	if err := c.Open(); err != nil {
		return "", err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rw == nil || c.rd == nil {
		return "", errors.New("servo serial is closed")
	}

	command := strings.TrimSpace(line)
	c.drainLocked(120 * time.Millisecond)

	if _, err := fmt.Fprintf(c.rw, "%s\n", command); err != nil {
		return "", err
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = c.rw.SetReadDeadline(deadline)
		defer c.rw.SetReadDeadline(time.Time{})
	}

	var lastLine string
	for {
		resp, err := c.rd.ReadString('\n')
		resp = strings.TrimSpace(resp)
		if resp != "" {
			lastLine = resp
			if isExpectedResponse(command, resp) {
				return resp, nil
			}
		}
		if errors.Is(err, os.ErrDeadlineExceeded) || ctx.Err() != nil {
			if lastLine != "" {
				return lastLine, fmt.Errorf("timed out waiting for response to %q; last serial line: %q", command, lastLine)
			}
			return "", ctx.Err()
		}
		if err != nil {
			return lastLine, err
		}
	}
}

func (c *Client) commandStatus(ctx context.Context, line string) (Status, error) {
	resp, err := c.Raw(ctx, line)
	if err != nil {
		return Status{}, err
	}
	st, err := ParseStatus(resp)
	if err != nil {
		return Status{Raw: resp}, err
	}
	return st, nil
}

func ParseStatus(line string) (Status, error) {
	raw := strings.TrimSpace(line)
	raw = strings.TrimPrefix(raw, "ok ")
	matches := statusPattern.FindStringSubmatch(raw)
	if len(matches) != 5 {
		return Status{Raw: line}, fmt.Errorf("unexpected servo status: %q", line)
	}
	values := make([]float64, 4)
	for i := range values {
		v, err := strconv.ParseFloat(matches[i+1], 64)
		if err != nil {
			return Status{Raw: line}, err
		}
		values[i] = v
	}
	return Status{
		Pan:        values[0],
		PanTarget:  values[1],
		Tilt:       values[2],
		TiltTarget: values[3],
		Raw:        line,
	}, nil
}

func configureSerial(device string, baud int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "stty", "-F", device, strconv.Itoa(baud), "raw", "-echo", "-icanon", "min", "0", "time", "5")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("configure serial with stty: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func (c *Client) drainLocked(duration time.Duration) {
	if c.rw == nil || c.rd == nil {
		return
	}
	deadline := time.Now().Add(duration)
	_ = c.rw.SetReadDeadline(deadline)
	defer c.rw.SetReadDeadline(time.Time{})
	for {
		_, err := c.rd.ReadString('\n')
		if err != nil {
			return
		}
		if time.Now().After(deadline) {
			return
		}
	}
}

func isExpectedResponse(command, response string) bool {
	cmd := commandName(command)
	line := strings.TrimSpace(response)
	if line == "" || strings.HasPrefix(line, "ESP32-C3 gimbal ready") || strings.HasPrefix(line, "initial:") {
		return false
	}

	switch cmd {
	case "status", "stat":
		return statusPattern.MatchString(line)
	case "center", "home", "c":
		return strings.HasPrefix(line, "ok center ") && statusPattern.MatchString(line)
	case "pan", "tilt", "move", "goto", "m", "step", "rel", "r":
		return strings.HasPrefix(line, "ok ") && statusPattern.MatchString(line)
	case "help", "?":
		return strings.HasPrefix(line, "commands:")
	case "laser":
		return strings.HasPrefix(line, "ok laser ") || strings.HasPrefix(line, "error: laser ")
	default:
		return true
	}
}

func commandName(command string) string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func Clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
