def parse_float(value):
    try:
        return float(value)
    except ValueError:
        return None


class CommandHandler:
    def __init__(self, gimbal, outputs=None):
        self.gimbal = gimbal
        self.outputs = outputs

    def handle(self, line):
        parts = line.strip().lower().replace(",", " ").split()
        if not parts:
            return None

        cmd = parts[0]
        speed = None
        values = []

        for part in parts[1:]:
            if part.startswith("s=") or part.startswith("speed="):
                speed = parse_float(part.split("=", 1)[1])
            else:
                values.append(part)

        if cmd in ("help", "?"):
            return (
                "commands: center | status | pan <0-270> [s=90] | tilt <0-180> [s=90] | "
                "move <pan> <tilt> [s=90] | step <dpan> <dtilt> [s=90] | "
                "laser on|off|pulse <ms>"
            )

        if cmd in ("center", "home", "c"):
            self.gimbal.center(speed)
            return "ok center " + self.gimbal.status()

        if cmd in ("status", "stat"):
            return self.gimbal.status()

        if cmd == "pan" and values:
            pan = parse_float(values[0])
            if pan is None:
                return "error: pan angle must be a number"
            self.gimbal.move_to(pan=pan, speed=speed)
            return "ok " + self.gimbal.status()

        if cmd == "tilt" and values:
            tilt = parse_float(values[0])
            if tilt is None:
                return "error: tilt angle must be a number"
            self.gimbal.move_to(tilt=tilt, speed=speed)
            return "ok " + self.gimbal.status()

        if cmd in ("move", "goto", "m") and len(values) >= 2:
            pan = parse_float(values[0])
            tilt = parse_float(values[1])
            if pan is None or tilt is None:
                return "error: move needs numeric pan and tilt"
            self.gimbal.move_to(pan=pan, tilt=tilt, speed=speed)
            return "ok " + self.gimbal.status()

        if cmd in ("step", "rel", "r") and len(values) >= 2:
            dpan = parse_float(values[0])
            dtilt = parse_float(values[1])
            if dpan is None or dtilt is None:
                return "error: step needs numeric dpan and dtilt"
            self.gimbal.move_by(dpan=dpan, dtilt=dtilt, speed=speed)
            return "ok " + self.gimbal.status()

        if cmd == "laser":
            return self._handle_laser(values)

        return "error: unknown command, type help"

    def _handle_laser(self, values):
        if self.outputs is None:
            return "error: outputs unavailable"
        if not values:
            return "error: laser needs on, off, or pulse"

        action = values[0]
        if action == "on":
            return self.outputs.laser_on()
        if action == "off":
            return self.outputs.laser_off()
        if action == "pulse" and len(values) >= 2:
            duration_ms = parse_float(values[1])
            if duration_ms is None:
                return "error: pulse duration must be a number"
            return self.outputs.laser_pulse(duration_ms)

        return "error: laser command must be on, off, or pulse <ms>"
