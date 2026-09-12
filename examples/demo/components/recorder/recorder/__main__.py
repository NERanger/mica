"""The Python recorder: subscribes to one event, publishes another.

It receives jobs.v1.JobCompleted from the C++ worker, summarizes it, and
publishes audit.v1.JobRecorded for anyone who cares about audit trails.
It imports only generated contract code and the MICA Python runtime --
never the worker's or the client's implementation.
"""

from __future__ import annotations

import asyncio
import logging
import time

from audit.v1.audit_pb2 import JobRecorded
from jobs.v1.jobs_pb2 import JobCompleted

from mica import App

logging.basicConfig(level=logging.INFO, format="%(message)s")
app = App("recorder")


# @app.subscribe registers an event handler for a generated message type.
# MICA derives the subject from the type; no strings involved.
@app.subscribe(JobCompleted)
async def on_completed(event: JobCompleted) -> None:
    print(
        f"received JobCompleted job={event.job_id.value} output={event.output}",
        flush=True,
    )
    recorded = JobRecorded()
    recorded.job_id = event.job_id.value
    recorded.summary = f"recorded {event.output}"
    recorded.timestamp_ns = time.time_ns()
    # publish() is fire-and-forget: zero or more subscribers may react.
    await app.publish(recorded)
    print("published JobRecorded", flush=True)


async def main() -> None:
    # Keep the process alive; the runtime handles shutdown on SIGINT.
    await asyncio.Event().wait()


if __name__ == "__main__":
    app.run(main)
