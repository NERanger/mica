from __future__ import annotations

import asyncio
import logging
import time

from camera.v1.camera_pb2 import PoseChanged
from tracking.v1.tracking_pb2 import PersonTracked

from mica import App

logging.basicConfig(level=logging.INFO, format="%(message)s")
app = App("tracker")


@app.subscribe(PoseChanged)
async def on_pose(event: PoseChanged) -> None:
    print(
        f"received PoseChanged pan={event.pose.pan} tilt={event.pose.tilt} zoom={event.pose.zoom}",
        flush=True,
    )
    tracked = PersonTracked()
    tracked.person_id.value = "person-1"
    tracked.x = event.pose.pan
    tracked.y = event.pose.tilt
    tracked.timestamp_ns = time.time_ns()
    await app.publish(tracked)
    print("published PersonTracked", flush=True)


async def main() -> None:
    await asyncio.Event().wait()


if __name__ == "__main__":
    app.run(main)
