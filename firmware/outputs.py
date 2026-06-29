from machine import Pin
import time

from config import ENABLE_LASER_OUTPUT, LASER_ACTIVE_HIGH, LASER_PIN


class DigitalOutput:
    def __init__(self, pin, active_high=True):
        self.active_high = active_high
        self.pin = Pin(pin, Pin.OUT)
        self.off_at_ms = None
        self.off()

    def _level(self, active):
        if self.active_high:
            return 1 if active else 0
        return 0 if active else 1

    def on(self):
        self.pin.value(self._level(True))

    def off(self):
        self.pin.value(self._level(False))
        self.off_at_ms = None

    def pulse(self, duration_ms):
        self.on()
        self.off_at_ms = time.ticks_add(time.ticks_ms(), max(0, int(duration_ms)))

    def update(self):
        if self.off_at_ms is None:
            return
        if time.ticks_diff(time.ticks_ms(), self.off_at_ms) >= 0:
            self.off()


class OutputBoard:
    def __init__(self):
        self.laser = None
        if ENABLE_LASER_OUTPUT:
            self.laser = DigitalOutput(LASER_PIN, LASER_ACTIVE_HIGH)

    def laser_on(self):
        if self.laser is None:
            return "error: laser output disabled"
        self.laser.on()
        return "ok laser on"

    def laser_off(self):
        if self.laser is None:
            return "error: laser output disabled"
        self.laser.off()
        return "ok laser off"

    def laser_pulse(self, duration_ms):
        if self.laser is None:
            return "error: laser output disabled"
        self.laser.pulse(duration_ms)
        return "ok laser pulse {}ms".format(int(duration_ms))

    def update(self):
        if self.laser is not None:
            self.laser.update()
