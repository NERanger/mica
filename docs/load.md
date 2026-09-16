# Communication load

Localhost NATS Core pressure tests for event hop latency, RPC RTT, and the demo pipeline. They do not change runtime APIs, contracts, or handler concurrency.

Handlers remain serial per subscription. There are no queue groups, retries, or pending-limit knobs.

## Profiles

| | Smoke | Heavy |
| --- | --- | --- |
| Command | `./scripts/test` (pytest default) | `./scripts/pressure` |
| Event rate | 1000/s | 1000/s |
| RPC rate | 200/s, 20 in-flight | 200/s, 20 in-flight |
| Duration | 2 s | 15 s |
| Matrix | Py→C++ / C++→Py / Go→Py events; Py→C++ / Py→Go / Go→Py RPC; Py client + C++ worker + Py recorder | Remaining language pairs, Py↔Py, event fan-out, Go client pipeline |

`MICA_LOAD=1 python3 -m pytest -m load` is the same as `./scripts/pressure`. Default pytest deselects `@pytest.mark.load`.

## SLOs

| Metric | Limit |
| --- | --- |
| Event hop p99 | < 20 ms |
| RPC RTT p99 | < 20 ms |
| Pipeline (call start → `JobRecorded`) p99 | < 50 ms |
| Loss, TIMEOUT, UNAVAILABLE | 0 |

The first 100 samples are discarded as warmup. Event hops use application `timestamp_ns` on `JobCompleted` / `JobRecorded`. RPC RTT is the caller clock around `call`. Envelope `timestamp_ns` is not visible to handlers.

## Harness commands

`mica-cpp-harness` and `mica-go-harness`:

```
publish-load <count> <rate>
subscribe-load <expect> <timeout_s>
call-load <count> <rate> <concurrency>
serve-pipeline
```

Each load command prints one `MICA_LOAD {json}` line to stdout. RPC servers keep using `serve-run ok`.

## Failures

A miss usually means a blocked handler, a slow subscriber, or a loaded machine. Do not change handler dispatch, add queue groups, or expose envelope timestamps without a design decision. See `docs/runtime-semantics.md`.
