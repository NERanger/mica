from __future__ import annotations

import asyncio
import logging
import signal
import time
from collections.abc import Awaitable, Callable
from typing import Any

import nats
from nats.errors import NoRespondersError, TimeoutError as NatsTimeoutError

from mica.config import AppConfig, PROTOCOL_VERSION
from mica.envelope import decode, encode, status_error
from mica.errors import (
    CallCancelled,
    CallTimeout,
    ProtocolError,
    RpcCode,
    RpcError,
    TransportError,
    is_wire_error,
)
from mica.health import Health, State
from mica.method import RpcMethod
from mica.subject import event_subject, rpc_subject
from mica.surface import Surface

logger = logging.getLogger("mica")

EventHandler = Callable[[Any], Awaitable[None] | None]
RpcHandler = Callable[[Any], Awaitable[Any] | Any]


class App:
    def __init__(self, name: str, config: AppConfig | None = None) -> None:
        self._config = config or AppConfig.from_env(name)
        self._name = self._config.name
        self._nc: Any = None
        self._state = State.STOPPED
        self._started_at: float | None = None
        self._event_handlers: list[tuple[type, EventHandler]] = []
        self._rpc_handlers: list[tuple[RpcMethod, RpcHandler]] = []
        self._subscriptions: list[Any] = []
        self._surface = Surface()
        self._lock = asyncio.Lock()

    def subscribe(self, event_type: type) -> Callable[[EventHandler], EventHandler]:
        def decorator(handler: EventHandler) -> EventHandler:
            self._event_handlers.append((event_type, handler))
            self._surface.record("subscribes", event_type.DESCRIPTOR.full_name)
            return handler

        return decorator

    def serve(self, method: RpcMethod) -> Callable[[RpcHandler], RpcHandler]:
        def decorator(handler: RpcHandler) -> RpcHandler:
            self._rpc_handlers.append((method, handler))
            self._surface.record("provides", method.contract_id)
            return handler

        return decorator

    async def start(self) -> None:
        if self._state == State.RUNNING:
            return
        self._state = State.STARTING

        async def error_cb(exc: Exception) -> None:
            if self._state in (State.STOPPING, State.STOPPED):
                return
            logger.error("transport error: %s", exc)

        try:
            self._nc = await asyncio.wait_for(
                nats.connect(
                    self._config.transport_url,
                    connect_timeout=1,
                    allow_reconnect=False,
                    max_reconnect_attempts=0,
                    dont_randomize=True,
                    error_cb=error_cb,
                ),
                timeout=3,
            )
        except Exception as exc:
            self._state = State.FAILED
            raise TransportError(
                f"failed to connect to transport at {self._config.transport_url}"
            ) from exc
        self._started_at = time.monotonic()
        for event_type, handler in self._event_handlers:
            await self._bind_event(event_type, handler)
        for method, handler in self._rpc_handlers:
            await self._bind_rpc(method, handler)
        self._state = State.RUNNING
        self._write_surface()

    async def shutdown(self) -> None:
        if self._state in (State.STOPPED, State.STOPPING):
            self._state = State.STOPPED
            return
        self._state = State.STOPPING
        try:
            if self._nc is not None:
                try:
                    await self._nc.drain()
                except Exception:
                    try:
                        await self._nc.close()
                    except Exception:
                        pass
        finally:
            self._nc = None
            self._subscriptions.clear()
            self._state = State.STOPPED
            self._write_surface()

    def run(self, main: Callable[[], Awaitable[None]] | None = None) -> None:
        asyncio.run(self._run(main))

    async def _run(self, main: Callable[[], Awaitable[None]] | None) -> None:
        loop = asyncio.get_running_loop()
        stop = asyncio.Event()

        def request_stop() -> None:
            stop.set()

        for sig in (signal.SIGINT, signal.SIGTERM):
            try:
                loop.add_signal_handler(sig, request_stop)
            except NotImplementedError:
                signal.signal(sig, lambda *_: request_stop())
        await self.start()
        try:
            if main is not None:
                main_task = asyncio.create_task(main())
                stop_task = asyncio.create_task(stop.wait())
                done, pending = await asyncio.wait(
                    {main_task, stop_task},
                    return_when=asyncio.FIRST_COMPLETED,
                )
                for task in pending:
                    task.cancel()
                if main_task in done:
                    main_task.result()
            else:
                await stop.wait()
        finally:
            await self.shutdown()

    async def publish(self, event: Any) -> None:
        self._require_running()
        desc = event.DESCRIPTOR
        contract_id = desc.full_name
        self._surface.record("publishes", contract_id)
        subject = event_subject(contract_id)
        body = encode(
            event.SerializeToString(),
            contract_id=contract_id,
            sender_id=self._name,
        )
        await self._nc.publish(subject, body)

    async def call(
        self,
        method: RpcMethod,
        request: Any,
        timeout: float | None = None,
    ) -> Any:
        self._require_running()
        self._surface.record("calls", method.contract_id)
        timeout_s = self._config.rpc_timeout_s if timeout is None else timeout
        subject = rpc_subject(method.service, method.method)
        body = encode(
            request.SerializeToString(),
            contract_id=method.contract_id,
            sender_id=self._name,
        )
        try:
            msg = await self._nc.request(subject, body, timeout=timeout_s)
        except NatsTimeoutError as exc:
            raise CallTimeout("rpc timed out") from exc
        except NoRespondersError as exc:
            raise RpcError(RpcCode.UNAVAILABLE, "no rpc provider") from exc
        except asyncio.CancelledError:
            raise CallCancelled("rpc cancelled") from None
        except Exception as exc:
            raise TransportError("rpc transport failure") from exc
        env = decode(msg.data)
        err = status_error(env)
        if err is not None:
            raise err
        response = method.response_type()
        try:
            response.ParseFromString(env.payload)
        except Exception as exc:
            raise ProtocolError("malformed rpc response payload") from exc
        return response

    def health(self) -> Health:
        uptime = 0
        if self._started_at is not None and self._state == State.RUNNING:
            uptime = int((time.monotonic() - self._started_at) * 1000)
        return Health(state=self._state, message="", uptime_ms=uptime)

    def _write_surface(self) -> None:
        path = self._config.surface_file
        if not path:
            return
        try:
            self._surface.write(path, self._name)
        except Exception:
            logger.exception("failed to write contract surface to %s", path)

    def _require_running(self) -> None:
        if self._state != State.RUNNING or self._nc is None:
            raise TransportError("app is not running")

    async def _bind_event(self, event_type: type, handler: EventHandler) -> None:
        contract_id = event_type.DESCRIPTOR.full_name
        subject = event_subject(contract_id)

        async def on_msg(msg: Any) -> None:
            try:
                env = decode(msg.data)
                if env.protocol_version != PROTOCOL_VERSION:
                    raise ProtocolError("protocol version mismatch")
                event = event_type()
                event.ParseFromString(env.payload)
            except Exception:
                logger.exception("dropping malformed event on %s", subject)
                return
            try:
                result = handler(event)
                if asyncio.iscoroutine(result):
                    await result
            except Exception:
                logger.exception("event handler failed for %s", contract_id)

        sub = await self._nc.subscribe(subject, cb=on_msg)
        self._subscriptions.append(sub)

    async def _bind_rpc(self, method: RpcMethod, handler: RpcHandler) -> None:
        subject = rpc_subject(method.service, method.method)

        async def on_msg(msg: Any) -> None:
            code = RpcCode.OK
            message = ""
            payload = b""
            try:
                env = decode(msg.data)
                request = method.request_type()
                request.ParseFromString(env.payload)
            except Exception:
                logger.exception("malformed rpc request on %s", subject)
                code = RpcCode.INVALID_ARGUMENT
                message = "malformed request"
            else:
                try:
                    result = handler(request)
                    if asyncio.iscoroutine(result):
                        result = await result
                    payload = result.SerializeToString()
                except RpcError as exc:
                    if is_wire_error(exc.code):
                        code = exc.code
                        message = exc.message
                    else:
                        logger.exception("rpc handler failed for %s", method.contract_id)
                        code = RpcCode.INTERNAL
                        message = "internal error"
                    payload = b""
                except Exception:
                    logger.exception("rpc handler failed for %s", method.contract_id)
                    code = RpcCode.INTERNAL
                    message = "internal error"
            if not msg.reply:
                return
            body = encode(
                payload,
                contract_id=method.contract_id,
                sender_id=self._name,
                code=code,
                message=message,
            )
            try:
                await self._nc.publish(msg.reply, body)
            except Exception:
                logger.exception("failed to publish rpc reply for %s", method.contract_id)

        sub = await self._nc.subscribe(subject, cb=on_msg)
        self._subscriptions.append(sub)
