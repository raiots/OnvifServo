#!/usr/bin/env python3
import os
import select
import sys
import termios
import time


def configure(fd, baud):
    attrs = termios.tcgetattr(fd)
    speed = getattr(termios, f"B{baud}")
    attrs[0] = 0
    attrs[1] = 0
    attrs[2] = termios.CS8 | termios.CREAD | termios.CLOCAL
    attrs[3] = 0
    attrs[4] = speed
    attrs[5] = speed
    attrs[6][termios.VMIN] = 0
    attrs[6][termios.VTIME] = 0
    termios.tcsetattr(fd, termios.TCSANOW, attrs)


def main():
    if len(sys.argv) < 2:
        print("usage: probe_uart.py /dev/ttyS2 [baud]", file=sys.stderr)
        return 2
    dev = sys.argv[1]
    baud = int(sys.argv[2]) if len(sys.argv) > 2 else 115200
    fd = os.open(dev, os.O_RDWR | os.O_NOCTTY | os.O_NONBLOCK)
    try:
        configure(fd, baud)
        termios.tcflush(fd, termios.TCIOFLUSH)
        os.write(fd, b"status\n")
        deadline = time.monotonic() + 2.0
        chunks = []
        while time.monotonic() < deadline:
            ready, _, _ = select.select([fd], [], [], 0.2)
            if not ready:
                continue
            data = os.read(fd, 256)
            if data:
                chunks.append(data)
                if b"\n" in data:
                    break
        if chunks:
            print(b"".join(chunks).decode(errors="replace").strip())
            return 0
        print("timeout: no UART response")
        return 1
    finally:
        os.close(fd)


if __name__ == "__main__":
    raise SystemExit(main())
