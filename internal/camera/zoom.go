package camera

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"onvif-servo-proxy/internal/config"
)

type ZoomController interface {
	Continuous(ctx context.Context, velocity float64) error
	Absolute(ctx context.Context, position float64) error
	Relative(ctx context.Context, delta float64) error
	Stop(ctx context.Context) error
	Status(ctx context.Context) (float64, bool, error)
}

type Controller struct {
	cfg    config.CameraConfig
	client *http.Client
	mu     sync.Mutex
	zoom   float64
	moving bool
}

func NewController(cfg config.CameraConfig) *Controller {
	if cfg.ZoomMax == cfg.ZoomMin {
		cfg.ZoomMax = 1
	}
	return &Controller{
		cfg: cfg,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
		zoom: cfg.ZoomDefault,
	}
}

func (c *Controller) Continuous(ctx context.Context, velocity float64) error {
	if c.cfg.ZoomMode == "disabled" || velocity == 0 {
		return c.Stop(ctx)
	}

	c.mu.Lock()
	c.moving = true
	if velocity > 0 {
		c.zoom = clamp(c.zoom+0.05, c.cfg.ZoomMin, c.cfg.ZoomMax)
	} else {
		c.zoom = clamp(c.zoom-0.05, c.cfg.ZoomMin, c.cfg.ZoomMax)
	}
	c.mu.Unlock()

	if c.cfg.ZoomMode == "http" {
		if velocity > 0 && c.cfg.HTTPZoomInURL != "" {
			return c.call(ctx, c.cfg.HTTPZoomInURL)
		}
		if velocity < 0 && c.cfg.HTTPZoomOutURL != "" {
			return c.call(ctx, c.cfg.HTTPZoomOutURL)
		}
	}
	if c.cfg.ZoomMode == "hikvision_isapi" {
		return c.hikvisionContinuous(ctx, velocity)
	}
	return nil
}

func (c *Controller) Absolute(ctx context.Context, position float64) error {
	c.mu.Lock()
	c.zoom = clamp(position, c.cfg.ZoomMin, c.cfg.ZoomMax)
	c.moving = true
	c.mu.Unlock()

	defer c.markIdleSoon()
	if c.cfg.ZoomMode == "disabled" {
		return nil
	}
	if c.cfg.ZoomMode == "http" && c.cfg.HTTPZoomSetURL != "" {
		url := strings.ReplaceAll(c.cfg.HTTPZoomSetURL, "{zoom}", fmt.Sprintf("%.4f", position))
		return c.call(ctx, url)
	}
	if c.cfg.ZoomMode == "onvif" {
		return errors.New("onvif zoom backend is reserved for the next adapter pass")
	}
	if c.cfg.ZoomMode == "hikvision_isapi" {
		return c.hikvisionContinuous(ctx, position-c.cfg.ZoomDefault)
	}
	return nil
}

func (c *Controller) Relative(ctx context.Context, delta float64) error {
	c.mu.Lock()
	target := clamp(c.zoom+delta, c.cfg.ZoomMin, c.cfg.ZoomMax)
	c.mu.Unlock()
	return c.Absolute(ctx, target)
}

func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.moving = false
	c.mu.Unlock()

	if c.cfg.ZoomMode == "http" && c.cfg.HTTPZoomStopURL != "" {
		return c.call(ctx, c.cfg.HTTPZoomStopURL)
	}
	if c.cfg.ZoomMode == "hikvision_isapi" {
		return c.hikvisionStop(ctx)
	}
	return nil
}

func (c *Controller) Status(ctx context.Context) (float64, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.zoom, c.moving, nil
}

func (c *Controller) call(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if c.cfg.HTTPAuthHeader != "" {
		req.Header.Set("Authorization", c.cfg.HTTPAuthHeader)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("camera zoom HTTP status %s", resp.Status)
	}
	return nil
}

func (c *Controller) hikvisionContinuous(ctx context.Context, velocity float64) error {
	speed := c.cfg.HikvisionZoomSpeed
	if speed <= 0 {
		speed = 60
	}
	if speed > 100 {
		speed = 100
	}
	zoom := int(clamp(velocity, -1, 1) * float64(speed))
	return c.hikvisionPTZ(ctx, zoom)
}

func (c *Controller) hikvisionStop(ctx context.Context) error {
	return c.hikvisionPTZ(ctx, 0)
}

func (c *Controller) hikvisionPTZ(ctx context.Context, zoom int) error {
	endpoint := strings.TrimRight(c.cfg.ONVIFEndpoint, "/")
	if endpoint == "" {
		return errors.New("hikvision_isapi zoom requires camera.onvif_endpoint")
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return err
	}
	channel := c.cfg.HikvisionChannel
	if channel == 0 {
		channel = 1
	}
	u.Path = fmt.Sprintf("/ISAPI/PTZCtrl/channels/%d/continuous", channel)
	u.RawQuery = ""
	body := fmt.Sprintf("<PTZData><pan>0</pan><tilt>0</tilt><zoom>%d</zoom></PTZData>", zoom)
	return c.digestRequest(ctx, http.MethodPut, u.String(), "application/xml", []byte(body))
}

func (c *Controller) digestRequest(ctx context.Context, method, reqURL, contentType string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		return checkStatus(resp)
	}
	challenge := resp.Header.Get("WWW-Authenticate")
	auth, err := digestAuthHeader(method, req.URL.RequestURI(), c.cfg.ONVIFUsername, c.cfg.ONVIFPassword, challenge)
	if err != nil {
		return err
	}
	req, err = http.NewRequestWithContext(ctx, method, reqURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Authorization", auth)
	resp, err = c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return checkStatus(resp)
}

func checkStatus(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("camera HTTP status %s", resp.Status)
	}
	return nil
}

var digestParamPattern = regexp.MustCompile(`([a-zA-Z]+)="?([^",]+)"?`)

func digestAuthHeader(method, uri, username, password, challenge string) (string, error) {
	if !strings.HasPrefix(challenge, "Digest ") {
		return "", fmt.Errorf("unsupported auth challenge: %s", challenge)
	}
	params := map[string]string{}
	for _, match := range digestParamPattern.FindAllStringSubmatch(challenge, -1) {
		params[match[1]] = match[2]
	}
	realm := params["realm"]
	nonce := params["nonce"]
	qop := params["qop"]
	if realm == "" || nonce == "" {
		return "", fmt.Errorf("invalid digest challenge: %s", challenge)
	}
	cnonce := randomHex(8)
	nc := "00000001"
	ha1 := md5Hex(username + ":" + realm + ":" + password)
	ha2 := md5Hex(method + ":" + uri)
	var response string
	if qop != "" {
		qop = "auth"
		response = md5Hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
	} else {
		response = md5Hex(ha1 + ":" + nonce + ":" + ha2)
	}
	parts := []string{
		`Digest username="` + username + `"`,
		`realm="` + realm + `"`,
		`nonce="` + nonce + `"`,
		`uri="` + uri + `"`,
		`response="` + response + `"`,
		`algorithm=MD5`,
	}
	if qop != "" {
		parts = append(parts, `qop=`+qop, `nc=`+nc, `cnonce="`+cnonce+`"`)
	}
	return strings.Join(parts, ", "), nil
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(buf)
}

func (c *Controller) markIdleSoon() {
	go func() {
		time.Sleep(400 * time.Millisecond)
		c.mu.Lock()
		c.moving = false
		c.mu.Unlock()
	}()
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
