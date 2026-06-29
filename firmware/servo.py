from machine import Pin, PWM

from config import PWM_FREQ_HZ, SERVO_MAX_US, SERVO_MIN_US


def clamp(value, low, high):
    return max(low, min(high, value))


class Servo:
    def __init__(self, pin, min_angle, max_angle, min_us=SERVO_MIN_US, max_us=SERVO_MAX_US):
        self.min_angle = min_angle
        self.max_angle = max_angle
        self.min_us = min_us
        self.max_us = max_us
        self.pwm = PWM(Pin(pin), freq=PWM_FREQ_HZ)

    def angle_to_pulse_us(self, angle):
        angle = clamp(angle, self.min_angle, self.max_angle)
        span = self.max_angle - self.min_angle
        ratio = (angle - self.min_angle) / span
        return int(self.min_us + ratio * (self.max_us - self.min_us))

    def write_angle(self, angle):
        pulse_us = self.angle_to_pulse_us(angle)
        if hasattr(self.pwm, "duty_ns"):
            self.pwm.duty_ns(pulse_us * 1000)
        elif hasattr(self.pwm, "duty_u16"):
            period_us = 1000000 // PWM_FREQ_HZ
            duty = int((pulse_us / period_us) * 65535)
            self.pwm.duty_u16(duty)
        else:
            period_us = 1000000 // PWM_FREQ_HZ
            duty = int((pulse_us / period_us) * 1023)
            self.pwm.duty(duty)

    def deinit(self):
        self.pwm.deinit()
