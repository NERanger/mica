// The Go client: drives the demo loop.
//
// It calls the jobs.v1.Worker.Run RPC on the C++ worker, then waits for
// audit.v1.JobRecorded from the Python recorder before the next
// iteration. The loop is bounded to three calls so the demo finishes on
// its own. It imports generated contract code and the MICA Go runtime,
// never the other components' source.
package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	auditv1 "mica/examples/demo/generated/go/audit/v1"
	"mica/examples/demo/generated/go/mica/tokens"
	jobsv1 "mica/examples/demo/generated/go/jobs/v1"
	"mica/runtime/go/mica"
)

const maxCalls = 3

func main() {
	app := mica.NewApp("client")
	var mu sync.Mutex
	calls := 0

	runJob := func() {
		mu.Lock()
		if calls >= maxCalls {
			mu.Unlock()
			return
		}
		calls++
		n := calls
		mu.Unlock()
		req := &jobsv1.RunRequest{
			JobId: &jobsv1.JobId{Value: fmt.Sprintf("job-%d", n)},
			Input: fmt.Sprintf("task-%d", n),
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Call() is request/response with a deadline. The token WorkerRun
		// comes from generated code; no subject string appears here.
		resp, err := app.Call(ctx, tokens.WorkerRun, req)
		if err != nil {
			log.Printf("Run failed: %v", err)
			return
		}
		out := resp.(*jobsv1.RunResponse)
		fmt.Printf("Run RPC completed accepted=%v job=job-%d\n", out.Accepted, n)
	}

	// Subscribe() registers an event handler for a generated message type;
	// MICA derives the subject from it.
	app.Subscribe((*auditv1.JobRecorded)(nil), func(ctx context.Context, msg proto.Message) error {
		event := msg.(*auditv1.JobRecorded)
		fmt.Printf("received JobRecorded job=%s summary=%s\n", event.JobId, event.Summary)
		runJob()
		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
	// Give subscribers a moment to connect, then kick off iteration one.
	time.Sleep(500 * time.Millisecond)
	runJob()
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
