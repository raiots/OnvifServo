package onvif

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"onvif-servo-proxy/internal/config"
	"onvif-servo-proxy/internal/ptz"
)

type Server struct {
	cfg config.Config
	ptz ptz.Backend
	log *slog.Logger
}

func NewServer(cfg config.Config, backend ptz.Backend, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{cfg: cfg, ptz: backend, log: logger}
}

func (s *Server) Register(mux *http.ServeMux) {
	base := strings.TrimRight(s.cfg.Server.BasePath, "/")
	mux.HandleFunc(base+"/device_service", s.handleDevice)
	mux.HandleFunc(base+"/media_service", s.handleMedia)
	mux.HandleFunc(base+"/ptz_service", s.handlePTZ)
	mux.HandleFunc(base+"/snapshot", s.handleSnapshot)
}

func (s *Server) handleDevice(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	switch action(r, body) {
	case "GetDeviceInformation":
		s.respond(w, s.deviceInformation())
	case "GetCapabilities":
		s.respond(w, s.capabilities(r))
	case "GetServices":
		s.respond(w, s.services(r))
	case "GetSystemDateAndTime":
		s.respond(w, systemDateAndTime())
	default:
		s.fault(w, "ter:ActionNotSupported", "unsupported device action")
	}
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	switch action(r, body) {
	case "GetProfiles":
		s.respond(w, s.profiles())
	case "GetStreamUri", "GetStreamURI":
		s.respond(w, s.streamURI())
	case "GetSnapshotUri", "GetSnapshotURI":
		s.respond(w, s.snapshotURI(r))
	case "GetVideoSources":
		s.respond(w, s.videoSources())
	default:
		s.fault(w, "ter:ActionNotSupported", "unsupported media action")
	}
}

func (s *Server) handlePTZ(w http.ResponseWriter, r *http.Request) {
	body := readBody(r)
	act := action(r, body)
	req := parseMove(body)

	var err error
	var response string
	switch act {
	case "ContinuousMove":
		err = s.ptz.ContinuousMove(req)
		response = ptzEmptyResponse("ContinuousMoveResponse")
	case "RelativeMove":
		err = s.ptz.RelativeMove(req)
		response = ptzEmptyResponse("RelativeMoveResponse")
	case "AbsoluteMove":
		err = s.ptz.AbsoluteMove(req)
		response = ptzEmptyResponse("AbsoluteMoveResponse")
	case "Stop":
		panTilt, zoom := parseStop(body)
		err = s.ptz.Stop(panTilt, zoom)
		response = ptzEmptyResponse("StopResponse")
	case "GetStatus":
		var status ptz.Status
		status, err = s.ptz.Status()
		if err == nil {
			response = s.ptzStatus(status)
		}
	case "GetPresets":
		response = presets()
	case "GotoPreset", "GotoHomePosition":
		err = s.ptz.Home()
		response = ptzEmptyResponse(act + "Response")
	case "GetConfigurations":
		response = s.ptzConfigurations()
	case "GetConfiguration":
		response = s.ptzConfiguration()
	case "GetConfigurationOptions":
		response = s.ptzConfigurationOptions()
	case "GetServiceCapabilities":
		response = s.ptzServiceCapabilities()
	case "GetNodes":
		response = s.ptzNodes()
	default:
		s.fault(w, "ter:ActionNotSupported", "unsupported ptz action")
		return
	}

	if err != nil {
		s.log.Warn("ptz action failed", "action", act, "error", err)
		s.fault(w, "ter:Action", err.Error())
		return
	}
	s.respond(w, response)
}

func (s *Server) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Camera.SnapshotURI == "" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, s.cfg.Camera.SnapshotURI, http.StatusTemporaryRedirect)
}

func (s *Server) respond(w http.ResponseWriter, inner string) {
	w.Header().Set("Content-Type", "application/soap+xml; charset=utf-8")
	_, _ = fmt.Fprint(w, envelope(inner))
}

func ptzEmptyResponse(name string) string {
	return fmt.Sprintf(`<tptz:%s xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl"/>`, name)
}

func (s *Server) fault(w http.ResponseWriter, code, reason string) {
	w.WriteHeader(http.StatusInternalServerError)
	s.respond(w, fmt.Sprintf(`<s:Fault><s:Code><s:Value>%s</s:Value></s:Code><s:Reason><s:Text xml:lang="en">%s</s:Text></s:Reason></s:Fault>`, code, xmlEscape(reason)))
}

func (s *Server) deviceInformation() string {
	info := s.cfg.Server
	return fmt.Sprintf(`<tds:GetDeviceInformationResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
<tds:Manufacturer>%s</tds:Manufacturer>
<tds:Model>%s</tds:Model>
<tds:FirmwareVersion>%s</tds:FirmwareVersion>
<tds:SerialNumber>%s</tds:SerialNumber>
<tds:HardwareId>%s</tds:HardwareId>
</tds:GetDeviceInformationResponse>`,
		xmlEscape(info.Manufacturer), xmlEscape(info.Model), xmlEscape(info.FirmwareVersion),
		xmlEscape(info.SerialNumber), xmlEscape(info.HardwareID))
}

func (s *Server) capabilities(r *http.Request) string {
	base := s.baseURL(r)
	return fmt.Sprintf(`<tds:GetCapabilitiesResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<tds:Capabilities>
<tt:Device><tt:XAddr>%s/device_service</tt:XAddr></tt:Device>
<tt:Media><tt:XAddr>%s/media_service</tt:XAddr></tt:Media>
<tt:PTZ><tt:XAddr>%s/ptz_service</tt:XAddr></tt:PTZ>
</tds:Capabilities>
</tds:GetCapabilitiesResponse>`, base, base, base)
}

func (s *Server) services(r *http.Request) string {
	base := s.baseURL(r)
	return fmt.Sprintf(`<tds:GetServicesResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl">
<tds:Service><tds:Namespace>http://www.onvif.org/ver10/device/wsdl</tds:Namespace><tds:XAddr>%s/device_service</tds:XAddr><tds:Version><tt:Major xmlns:tt="http://www.onvif.org/ver10/schema">2</tt:Major><tt:Minor xmlns:tt="http://www.onvif.org/ver10/schema">0</tt:Minor></tds:Version></tds:Service>
<tds:Service><tds:Namespace>http://www.onvif.org/ver10/media/wsdl</tds:Namespace><tds:XAddr>%s/media_service</tds:XAddr><tds:Version><tt:Major xmlns:tt="http://www.onvif.org/ver10/schema">2</tt:Major><tt:Minor xmlns:tt="http://www.onvif.org/ver10/schema">0</tt:Minor></tds:Version></tds:Service>
<tds:Service><tds:Namespace>http://www.onvif.org/ver20/ptz/wsdl</tds:Namespace><tds:XAddr>%s/ptz_service</tds:XAddr><tds:Version><tt:Major xmlns:tt="http://www.onvif.org/ver10/schema">2</tt:Major><tt:Minor xmlns:tt="http://www.onvif.org/ver10/schema">0</tt:Minor></tds:Version></tds:Service>
</tds:GetServicesResponse>`, base, base, base)
}

func (s *Server) profiles() string {
	return `<trt:GetProfilesResponse xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<trt:Profiles token="profile_0" fixed="true">
<tt:Name>Servo PTZ Proxy</tt:Name>
<tt:VideoSourceConfiguration token="video_source_0"><tt:Name>Real Camera</tt:Name><tt:UseCount>1</tt:UseCount><tt:SourceToken>video_source_0</tt:SourceToken><tt:Bounds x="0" y="0" width="1920" height="1080"/></tt:VideoSourceConfiguration>
<tt:VideoEncoderConfiguration token="encoder_0"><tt:Name>Real Camera H264</tt:Name><tt:UseCount>1</tt:UseCount><tt:Encoding>H264</tt:Encoding><tt:Resolution><tt:Width>1920</tt:Width><tt:Height>1080</tt:Height></tt:Resolution><tt:Quality>80</tt:Quality><tt:RateControl><tt:FrameRateLimit>25</tt:FrameRateLimit><tt:EncodingInterval>1</tt:EncodingInterval><tt:BitrateLimit>4096</tt:BitrateLimit></tt:RateControl><tt:H264><tt:GovLength>25</tt:GovLength><tt:H264Profile>Main</tt:H264Profile></tt:H264><tt:SessionTimeout>PT60S</tt:SessionTimeout></tt:VideoEncoderConfiguration>
<tt:PTZConfiguration token="ptz_0"><tt:Name>Servo PTZ</tt:Name><tt:UseCount>1</tt:UseCount><tt:NodeToken>ptz_node_0</tt:NodeToken><tt:DefaultContinuousPanTiltVelocitySpace>http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace</tt:DefaultContinuousPanTiltVelocitySpace><tt:DefaultContinuousZoomVelocitySpace>http://www.onvif.org/ver10/tptz/ZoomSpaces/VelocityGenericSpace</tt:DefaultContinuousZoomVelocitySpace><tt:DefaultRelativePanTiltTranslationSpace>http://www.onvif.org/ver10/tptz/PanTiltSpaces/TranslationSpaceFov</tt:DefaultRelativePanTiltTranslationSpace><tt:DefaultPTZSpeed><tt:PanTilt x="0.5" y="0.5" space="http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace"/><tt:Zoom x="0.5" space="http://www.onvif.org/ver10/tptz/ZoomSpaces/VelocityGenericSpace"/></tt:DefaultPTZSpeed><tt:DefaultPTZTimeout>PT1S</tt:DefaultPTZTimeout></tt:PTZConfiguration>
</trt:Profiles>
</trt:GetProfilesResponse>`
}

func (s *Server) streamURI() string {
	uri := s.cfg.Camera.StreamURI
	return fmt.Sprintf(`<trt:GetStreamUriResponse xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<trt:MediaUri><tt:Uri>%s</tt:Uri><tt:InvalidAfterConnect>false</tt:InvalidAfterConnect><tt:InvalidAfterReboot>false</tt:InvalidAfterReboot><tt:Timeout>PT60S</tt:Timeout></trt:MediaUri>
</trt:GetStreamUriResponse>`, xmlEscape(uri))
}

func (s *Server) snapshotURI(r *http.Request) string {
	uri := s.cfg.Camera.SnapshotURI
	if uri == "" {
		uri = s.baseURL(r) + "/snapshot"
	}
	return fmt.Sprintf(`<trt:GetSnapshotUriResponse xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<trt:MediaUri><tt:Uri>%s</tt:Uri><tt:InvalidAfterConnect>false</tt:InvalidAfterConnect><tt:InvalidAfterReboot>false</tt:InvalidAfterReboot><tt:Timeout>PT5S</tt:Timeout></trt:MediaUri>
</trt:GetSnapshotUriResponse>`, xmlEscape(uri))
}

func (s *Server) videoSources() string {
	return `<trt:GetVideoSourcesResponse xmlns:trt="http://www.onvif.org/ver10/media/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<trt:VideoSources token="video_source_0"><tt:Framerate>25</tt:Framerate><tt:Resolution><tt:Width>1920</tt:Width><tt:Height>1080</tt:Height></tt:Resolution></trt:VideoSources>
</trt:GetVideoSourcesResponse>`
}

func (s *Server) ptzStatus(st ptz.Status) string {
	return fmt.Sprintf(`<tptz:GetStatusResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<tptz:PTZStatus>
<tt:Position><tt:PanTilt x="%.4f" y="%.4f" space="http://www.onvif.org/ver10/tptz/PanTiltSpaces/PositionGenericSpace"/><tt:Zoom x="%.4f" space="http://www.onvif.org/ver10/tptz/ZoomSpaces/PositionGenericSpace"/></tt:Position>
<tt:MoveStatus><tt:PanTilt>%s</tt:PanTilt><tt:Zoom>%s</tt:Zoom></tt:MoveStatus>
<tt:UtcTime>%s</tt:UtcTime>
</tptz:PTZStatus>
</tptz:GetStatusResponse>`,
		rangeToNormalized(st.Pan, s.cfg.Servo.PanMin, s.cfg.Servo.PanMax),
		rangeToNormalized(st.Tilt, s.cfg.Servo.TiltMin, s.cfg.Servo.TiltMax),
		st.Zoom,
		moveStatus(st.PanMoving || st.TiltMoving),
		moveStatus(st.ZoomMoving),
		time.Now().UTC().Format(time.RFC3339))
}

func presets() string {
	return `<tptz:GetPresetsResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<tptz:Preset token="home"><tt:Name>Home</tt:Name><tt:PTZPosition><tt:PanTilt x="0" y="0"/><tt:Zoom x="0"/></tt:PTZPosition></tptz:Preset>
</tptz:GetPresetsResponse>`
}

func (s *Server) ptzConfigurations() string {
	return `<tptz:GetConfigurationsResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
` + s.ptzConfigurationBody() + `
</tptz:GetConfigurationsResponse>`
}

func (s *Server) ptzConfiguration() string {
	return `<tptz:GetConfigurationResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
` + s.ptzConfigurationBody() + `
</tptz:GetConfigurationResponse>`
}

func (s *Server) ptzConfigurationBody() string {
	return `<tptz:PTZConfiguration token="ptz_0"><tt:Name>Servo PTZ</tt:Name><tt:UseCount>1</tt:UseCount><tt:NodeToken>ptz_node_0</tt:NodeToken><tt:DefaultContinuousPanTiltVelocitySpace>http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace</tt:DefaultContinuousPanTiltVelocitySpace><tt:DefaultContinuousZoomVelocitySpace>http://www.onvif.org/ver10/tptz/ZoomSpaces/VelocityGenericSpace</tt:DefaultContinuousZoomVelocitySpace><tt:DefaultRelativePanTiltTranslationSpace>http://www.onvif.org/ver10/tptz/PanTiltSpaces/TranslationSpaceFov</tt:DefaultRelativePanTiltTranslationSpace><tt:DefaultPTZSpeed><tt:PanTilt x="0.5" y="0.5" space="http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace"/><tt:Zoom x="0.5" space="http://www.onvif.org/ver10/tptz/ZoomSpaces/VelocityGenericSpace"/></tt:DefaultPTZSpeed><tt:DefaultPTZTimeout>PT1S</tt:DefaultPTZTimeout><tt:PanTiltLimits><tt:Range><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/PositionGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:Range></tt:PanTiltLimits><tt:ZoomLimits><tt:Range><tt:URI>http://www.onvif.org/ver10/tptz/ZoomSpaces/PositionGenericSpace</tt:URI><tt:XRange><tt:Min>0</tt:Min><tt:Max>1</tt:Max></tt:XRange></tt:Range></tt:ZoomLimits></tptz:PTZConfiguration>`
}

func (s *Server) ptzServiceCapabilities() string {
	return `<tptz:GetServiceCapabilitiesResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl">
<tptz:Capabilities EFlip="false" Reverse="false" GetCompatibleConfigurations="false" MoveStatus="true" StatusPosition="true"/>
</tptz:GetServiceCapabilitiesResponse>`
}

func (s *Server) ptzNodes() string {
	return `<tptz:GetNodesResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<tptz:PTZNode token="ptz_node_0"><tt:Name>Servo PTZ Node</tt:Name><tt:SupportedPTZSpaces><tt:AbsolutePanTiltPositionSpace><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/PositionGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:AbsolutePanTiltPositionSpace><tt:RelativePanTiltTranslationSpace><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/TranslationSpaceFov</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:RelativePanTiltTranslationSpace><tt:ContinuousPanTiltVelocitySpace><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:ContinuousPanTiltVelocitySpace></tt:SupportedPTZSpaces><tt:MaximumNumberOfPresets>16</tt:MaximumNumberOfPresets><tt:HomeSupported>true</tt:HomeSupported></tptz:PTZNode>
</tptz:GetNodesResponse>`
}

func (s *Server) ptzConfigurationOptions() string {
	return `<tptz:GetConfigurationOptionsResponse xmlns:tptz="http://www.onvif.org/ver20/ptz/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<tptz:PTZConfigurationOptions>
<tt:Spaces>
<tt:AbsolutePanTiltPositionSpace><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/PositionGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:AbsolutePanTiltPositionSpace>
<tt:RelativePanTiltTranslationSpace><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/TranslationSpaceFov</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:RelativePanTiltTranslationSpace>
<tt:ContinuousPanTiltVelocitySpace><tt:URI>http://www.onvif.org/ver10/tptz/PanTiltSpaces/VelocityGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange><tt:YRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:YRange></tt:ContinuousPanTiltVelocitySpace>
<tt:AbsoluteZoomPositionSpace><tt:URI>http://www.onvif.org/ver10/tptz/ZoomSpaces/PositionGenericSpace</tt:URI><tt:XRange><tt:Min>0</tt:Min><tt:Max>1</tt:Max></tt:XRange></tt:AbsoluteZoomPositionSpace>
<tt:RelativeZoomTranslationSpace><tt:URI>http://www.onvif.org/ver10/tptz/ZoomSpaces/TranslationGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange></tt:RelativeZoomTranslationSpace>
<tt:ContinuousZoomVelocitySpace><tt:URI>http://www.onvif.org/ver10/tptz/ZoomSpaces/VelocityGenericSpace</tt:URI><tt:XRange><tt:Min>-1</tt:Min><tt:Max>1</tt:Max></tt:XRange></tt:ContinuousZoomVelocitySpace>
</tt:Spaces>
</tptz:PTZConfigurationOptions>
</tptz:GetConfigurationOptionsResponse>`
}

func systemDateAndTime() string {
	now := time.Now().UTC()
	return fmt.Sprintf(`<tds:GetSystemDateAndTimeResponse xmlns:tds="http://www.onvif.org/ver10/device/wsdl" xmlns:tt="http://www.onvif.org/ver10/schema">
<tds:SystemDateAndTime><tt:DateTimeType>NTP</tt:DateTimeType><tt:DaylightSavings>false</tt:DaylightSavings><tt:UTCDateTime><tt:Time><tt:Hour>%d</tt:Hour><tt:Minute>%d</tt:Minute><tt:Second>%d</tt:Second></tt:Time><tt:Date><tt:Year>%d</tt:Year><tt:Month>%d</tt:Month><tt:Day>%d</tt:Day></tt:Date></tt:UTCDateTime></tds:SystemDateAndTime>
</tds:GetSystemDateAndTimeResponse>`, now.Hour(), now.Minute(), now.Second(), now.Year(), int(now.Month()), now.Day())
}

func (s *Server) baseURL(r *http.Request) string {
	host := s.cfg.Server.PublicHost
	if host == "" {
		host = r.Host
	}
	if !strings.Contains(host, ":") && s.cfg.Server.Port != 80 {
		host = host + ":" + strconv.Itoa(s.cfg.Server.Port)
	}
	return "http://" + host + strings.TrimRight(s.cfg.Server.BasePath, "/")
}

func envelope(inner string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>` +
		`<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope">` +
		`<s:Body>` + inner + `</s:Body></s:Envelope>`
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func moveStatus(moving bool) string {
	if moving {
		return "MOVING"
	}
	return "IDLE"
}

func rangeToNormalized(v, min, max float64) float64 {
	if max == min {
		return 0
	}
	return ((v - min) / (max - min) * 2) - 1
}
