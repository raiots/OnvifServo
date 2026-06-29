from machine import Pin, UART
import sys
import time

try:
    import select
except ImportError:
    select = None

try:
    import network
    import socket
except ImportError:
    network = None
    socket = None

from config import (
    ENABLE_UART_INPUT,
    ENABLE_WIFI_INPUT,
    UART_BAUD,
    UART_ID,
    UART_RX_PIN,
    UART_TX_PIN,
    UDP_PORT,
    WIFI_CONNECT_TIMEOUT_MS,
    WIFI_PASSWORD,
    WIFI_SSID,
)


class LineReader:
    def __init__(self, stream):
        self.stream = stream
        self.buffer = b""

    def poll(self):
        try:
            if hasattr(self.stream, "any") and not self.stream.any():
                return None
            data = self.stream.read(1)
        except Exception:
            return None

        if not data:
            return None

        if data in (b"\n", b"\r"):
            if not self.buffer:
                return None
            line = safe_text(self.buffer).strip()
            self.buffer = b""
            if not line:
                return None
            return line, None

        self.buffer += data
        if len(self.buffer) > 128:
            self.buffer = b""
        return None


def safe_text(data):
    return "".join(chr(ch) for ch in data if 32 <= ch <= 126)


class StdioInput:
    def __init__(self):
        if select is None:
            self.poller = None
        else:
            self.poller = select.poll()
            self.poller.register(sys.stdin, select.POLLIN)

    def poll(self):
        if self.poller is None or not self.poller.poll(0):
            return None
        try:
            return sys.stdin.readline().strip(), None
        except Exception:
            return None


class UartInput:
    def __init__(self):
        uart = UART(
            UART_ID,
            baudrate=UART_BAUD,
            tx=Pin(UART_TX_PIN),
            rx=Pin(UART_RX_PIN),
        )
        self.reader = LineReader(uart)

    def poll(self):
        return self.reader.poll()

    def reply(self, response, remote):
        try:
            self.reader.stream.write((response + "\n").encode())
        except Exception:
            pass


class UdpInput:
    def __init__(self):
        wlan = network.WLAN(network.STA_IF)
        wlan.active(True)
        wlan.connect(WIFI_SSID, WIFI_PASSWORD)
        start_ms = time.ticks_ms()
        while not wlan.isconnected():
            if time.ticks_diff(time.ticks_ms(), start_ms) > WIFI_CONNECT_TIMEOUT_MS:
                raise RuntimeError("wifi connection timeout")
            time.sleep_ms(200)

        addr = socket.getaddrinfo("0.0.0.0", UDP_PORT)[0][-1]
        self.sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        self.sock.bind(addr)
        self.sock.setblocking(False)
        print("wifi connected:", wlan.ifconfig()[0], "udp port", UDP_PORT)

    def poll(self):
        try:
            data, remote = self.sock.recvfrom(256)
        except OSError:
            return None
        if not data:
            return None
        line = safe_text(data).strip()
        if not line:
            return None
        return line, remote

    def reply(self, response, remote):
        if remote is None:
            return
        try:
            self.sock.sendto(response.encode(), remote)
        except OSError:
            pass


def make_inputs():
    inputs = [StdioInput()]

    if ENABLE_UART_INPUT:
        inputs.append(UartInput())

    if ENABLE_WIFI_INPUT and network is not None and socket is not None:
        try:
            inputs.append(UdpInput())
        except Exception as exc:
            print("wifi input disabled:", exc)

    return inputs
