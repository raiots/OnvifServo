from config import (
    BOTTOM_SERVO_PIN,
    DEFAULT_SPEED_DPS,
    PAN_CENTER,
    PAN_MAX,
    PAN_MIN,
    TILT_CENTER,
    TILT_MAX,
    TILT_MIN,
    TOP_SERVO_PIN,
)
from servo import Servo, clamp


class SmoothServo:
    def __init__(self, servo, initial_angle, speed_dps=DEFAULT_SPEED_DPS):
        self.servo = servo
        self.current = float(initial_angle)
        self.target = float(initial_angle)
        self.speed_dps = float(speed_dps)
        self.servo.write_angle(self.current)

    def set_target(self, angle, speed_dps=None):
        self.target = float(clamp(angle, self.servo.min_angle, self.servo.max_angle))
        if speed_dps is not None:
            self.speed_dps = max(1.0, float(speed_dps))

    def step(self, dt_s):
        delta = self.target - self.current
        if abs(delta) < 0.01:
            return False

        max_step = self.speed_dps * dt_s
        if abs(delta) <= max_step:
            self.current = self.target
        else:
            self.current += max_step if delta > 0 else -max_step

        self.servo.write_angle(self.current)
        return True


class Gimbal:
    def __init__(self):
        self.tilt = SmoothServo(
            Servo(TOP_SERVO_PIN, TILT_MIN, TILT_MAX),
            TILT_CENTER,
        )
        self.pan = SmoothServo(
            Servo(BOTTOM_SERVO_PIN, PAN_MIN, PAN_MAX),
            PAN_CENTER,
        )

    def move_to(self, pan=None, tilt=None, speed=None):
        if pan is not None:
            self.pan.set_target(pan, speed)
        if tilt is not None:
            self.tilt.set_target(tilt, speed)

    def move_by(self, dpan=0, dtilt=0, speed=None):
        self.move_to(
            pan=self.pan.target + dpan,
            tilt=self.tilt.target + dtilt,
            speed=speed,
        )

    def center(self, speed=None):
        self.move_to(pan=PAN_CENTER, tilt=TILT_CENTER, speed=speed)

    def update(self, dt_s):
        self.pan.step(dt_s)
        self.tilt.step(dt_s)

    def status(self):
        return "pan={:.1f}/{:.1f}, tilt={:.1f}/{:.1f}, speed pan={:.1f}, tilt={:.1f}".format(
            self.pan.current,
            self.pan.target,
            self.tilt.current,
            self.tilt.target,
            self.pan.speed_dps,
            self.tilt.speed_dps,
        )

    def deinit(self):
        self.pan.servo.deinit()
        self.tilt.servo.deinit()
