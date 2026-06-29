package onvif

import (
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"onvif-servo-proxy/internal/ptz"
)

var (
	actionPattern       = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?([A-Z][A-Za-z0-9]+)\b`)
	profileTokenPattern = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?ProfileToken>([^<]+)</`)
	panTiltPattern      = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?PanTilt\b[^>]*\bx="([^"]+)"[^>]*\by="([^"]+)"`)
	zoomPattern         = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?Zoom\b[^>]*\bx="([^"]+)"`)
	timeoutPattern      = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?Timeout>PT([0-9.]+)S</`)
	panTiltStopPattern  = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?PanTilt>(true|false|1|0)</`)
	zoomStopPattern     = regexp.MustCompile(`(?i)<(?:[a-z0-9]+:)?Zoom>(true|false|1|0)</`)
)

func readBody(r *http.Request) string {
	defer r.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	return string(data)
}

func action(r *http.Request, body string) string {
	header := strings.Trim(r.Header.Get("SOAPAction"), `"`)
	if header != "" {
		if idx := strings.LastIndexAny(header, "/#"); idx >= 0 && idx+1 < len(header) {
			return header[idx+1:]
		}
		return header
	}
	matches := actionPattern.FindAllStringSubmatch(body, -1)
	for _, match := range matches {
		name := match[1]
		if name != "Envelope" && name != "Header" && name != "Body" && name != "Security" {
			return name
		}
	}
	return ""
}

func parseMove(body string) ptz.MoveRequest {
	req := ptz.MoveRequest{
		ProfileToken: "profile_0",
		Timeout:      200 * time.Millisecond,
	}
	if matches := profileTokenPattern.FindStringSubmatch(body); len(matches) == 2 {
		req.ProfileToken = strings.TrimSpace(matches[1])
	}
	if matches := panTiltPattern.FindStringSubmatch(body); len(matches) == 3 {
		req.PanTilt = &ptz.Vector2D{X: parseFloat(matches[1]), Y: parseFloat(matches[2])}
	}
	if matches := zoomPattern.FindStringSubmatch(body); len(matches) == 2 {
		req.Zoom = &ptz.Vector1D{X: parseFloat(matches[1])}
	}
	if matches := timeoutPattern.FindStringSubmatch(body); len(matches) == 2 {
		req.Timeout = time.Duration(parseFloat(matches[1]) * float64(time.Second))
	}
	return req
}

func parseStop(body string) (bool, bool) {
	panTilt := parseBoolMatch(panTiltStopPattern.FindStringSubmatch(body))
	zoom := parseBoolMatch(zoomStopPattern.FindStringSubmatch(body))
	if !panTilt && !zoom {
		return true, true
	}
	return panTilt, zoom
}

func parseBoolMatch(matches []string) bool {
	if len(matches) != 2 {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(matches[1]))
	return v == "true" || v == "1"
}

func parseFloat(raw string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return v
}
