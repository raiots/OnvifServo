import time

from commands import CommandHandler
from config import UPDATE_INTERVAL_MS
from gimbal import Gimbal
from inputs import make_inputs
from outputs import OutputBoard


def main():
    gimbal = Gimbal()
    outputs = OutputBoard()
    handler = CommandHandler(gimbal, outputs)
    inputs = make_inputs()

    print("ESP32-C3 gimbal ready")
    print(handler.handle("help"))
    print("initial:", gimbal.status())

    last_ms = time.ticks_ms()
    try:
        while True:
            now_ms = time.ticks_ms()
            dt_ms = time.ticks_diff(now_ms, last_ms)
            if dt_ms >= UPDATE_INTERVAL_MS:
                last_ms = now_ms
                gimbal.update(dt_ms / 1000)
                outputs.update()

            for source in inputs:
                item = source.poll()
                if not item:
                    continue

                line, remote = item
                if not line:
                    continue

                response = handler.handle(line)
                if response:
                    print(response)
                    if hasattr(source, "reply"):
                        source.reply(response, remote)

            time.sleep_ms(2)
    finally:
        gimbal.deinit()


main()
