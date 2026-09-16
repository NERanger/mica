# C++ runtime

C++20. Handlers run on the nats.c callback thread. `call` blocks the calling thread until reply or timeout.

```cpp
mica::App app{"worker"};

app.subscribe<jobs::v1::JobCompleted>([](const jobs::v1::JobCompleted& event) {
  ...
});

app.serve(mica::tokens::Worker_Run,
          [](const jobs::v1::RunRequest& request) {
            jobs::v1::RunResponse response;
            return response;
          });

auto response = app.call(mica::tokens::Worker_Run, request, 1s);
app.run();
```

Throw `mica::RpcError` from a serve handler for a structured status. Other exceptions become `INTERNAL`.

`call` throws `CallTimeout` when the caller deadline expires. That is a local outcome, not wire status. C++ `call` has no cancellation API.

Event handler exceptions are caught and logged.

`run()` calls `start()` then waits for SIGINT/SIGTERM or `shutdown()`.

`AppConfig::from_env` reads `MICA_NATS_URL`, `MICA_COMPONENT_NAME`, and `MICA_SURFACE_FILE`. `MICA_SURFACE_FILE` enables contract surface reporting (see `docs/runtime-semantics.md`).

Default RPC timeout: 5 seconds. No retries.

Do not block nats callback threads longer than necessary. Long work in a subscribe handler delays other callbacks on that connection.
