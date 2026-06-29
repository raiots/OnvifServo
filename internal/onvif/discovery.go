package onvif

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"onvif-servo-proxy/internal/config"
)

func StartDiscovery(ctx context.Context, cfg config.Config, logger *slog.Logger) {
	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:3702")
	if err != nil {
		logger.Warn("resolve ws-discovery multicast", "error", err)
		return
	}
	conn, err := net.ListenMulticastUDP("udp4", nil, addr)
	if err != nil {
		logger.Warn("start ws-discovery responder", "error", err)
		return
	}
	defer conn.Close()
	_ = conn.SetReadBuffer(64 * 1024)
	logger.Info("ws-discovery responder listening", "addr", addr.String())

	buf := make([]byte, 64*1024)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				continue
			}
		}
		msg := string(buf[:n])
		if !strings.Contains(msg, "Probe") || !strings.Contains(msg, "NetworkVideoTransmitter") {
			continue
		}
		xaddr := discoveryXAddr(cfg)
		reply := discoveryProbeMatch(xaddr, cfg.Server)
		if _, err := conn.WriteToUDP([]byte(reply), remote); err != nil {
			logger.Warn("send ws-discovery response", "remote", remote.String(), "error", err)
		}
	}
}

func discoveryXAddr(cfg config.Config) string {
	host := cfg.Server.PublicHost
	if host == "" {
		host = localIPv4()
	}
	return fmt.Sprintf("http://%s:%d%s/device_service", host, cfg.Server.Port, strings.TrimRight(cfg.Server.BasePath, "/"))
}

func localIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ip, _, err := net.ParseCIDR(addr.String())
			if err == nil && ip.To4() != nil {
				return ip.String()
			}
		}
	}
	return "127.0.0.1"
}

func discoveryProbeMatch(xaddr string, server config.ServerConfig) string {
	uuid := "urn:uuid:" + strings.ToLower(strings.ReplaceAll(server.SerialNumber, "_", "-"))
	if server.SerialNumber == "" {
		uuid = "urn:uuid:onvif-servo-proxy"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:a="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:d="http://schemas.xmlsoap.org/ws/2005/04/discovery" xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
<s:Header>
<a:Action>http://schemas.xmlsoap.org/ws/2005/04/discovery/ProbeMatches</a:Action>
<a:MessageID>urn:uuid:onvif-servo-proxy-response</a:MessageID>
<a:RelatesTo>urn:uuid:onvif-servo-proxy-probe</a:RelatesTo>
<a:To>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</a:To>
</s:Header>
<s:Body>
<d:ProbeMatches>
<d:ProbeMatch>
<a:EndpointReference><a:Address>%s</a:Address></a:EndpointReference>
<d:Types>dn:NetworkVideoTransmitter</d:Types>
<d:Scopes>onvif://www.onvif.org/name/%s onvif://www.onvif.org/hardware/%s onvif://www.onvif.org/Profile/Streaming</d:Scopes>
<d:XAddrs>%s</d:XAddrs>
<d:MetadataVersion>1</d:MetadataVersion>
</d:ProbeMatch>
</d:ProbeMatches>
</s:Body>
</s:Envelope>`, uuid, xmlEscape(server.Model), xmlEscape(server.HardwareID), xaddr)
}
