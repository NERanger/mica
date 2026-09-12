# Runtime semantics

This document is language-neutral. Python, C++, and Go must conform to it.

## Protocol version

`mica.v1.Envelope.protocol_version` is `1`. This is the MICA protocol version, not an application contract version (`jobs.v1` vs `jobs.v2`).

Unknown protocol versions are malformed.

## Envelope

Every event and RPC payload is wrapped in `mica.v1.Envelope`:

- `protocol_version`
- `message_id`
- `sender_id`
- `timestamp_ns`
- `contract_id`
- `status` (RPC replies)
- `payload` (serialized application message)

## Subjects

Protobuf full names do not include a leading dot.

```
event_subject(message_full_name) = "event." + message_full_name
rpc_subject(service_full_name, method) = "rpc." + service_full_name + "." + method
```

Examples:

- `event.jobs.v1.JobCompleted`
- `rpc.jobs.v1.Worker.Run`

Languages must produce byte-for-byte identical subjects. Golden cases live in `tests/compatibility/cases.toml`.

## Events

`publish` sends one envelope to the event subject. There is no application-level response.

`subscribe` receives envelopes, unpacks the payload, and invokes the handler.

Malformed envelopes or payloads are logged and dropped. The process stays running.

Handler failures are logged. They do not unsubscribe and do not fail the process.

## RPC

`call` sends a request envelope using NATS Core request/reply.

`serve` answers on the NATS reply subject with an envelope containing `RpcStatus` and, on success, the response payload.

Default timeout is 5 seconds. Timeouts are explicit. V0 does not retry.

Codes:

| Code | Meaning |
| --- | --- |
| OK | success |
| INVALID_ARGUMENT | bad request or handler rejection |
| NOT_FOUND | requested entity missing |
| UNAVAILABLE | no provider / transport cannot complete |
| INTERNAL | handler panic/exception; wire message is generic |
| TIMEOUT | caller deadline expired |
| CANCELLED | caller cancelled locally |

Language exceptions, tracebacks, and concrete error types are not part of the wire format.

## Cancellation

Caller cancellation is local. NATS Core does not cancel the server handler. A late reply is ignored by a timed-out client. MICA does not claim distributed cancellation.

## Lifecycle

`STARTING` → `RUNNING` → `STOPPING` → `STOPPED`, or `FAILED` on startup failure.

Graceful shutdown stops new work where practical, drains the NATS connection, and exits.

## Health

`health()` is local: state, message, uptime. V0 has no distributed health mesh. The launcher tracks child liveness separately.

## Concurrency

Threading models differ by language and are documented per runtime. Wire behavior is the same.
