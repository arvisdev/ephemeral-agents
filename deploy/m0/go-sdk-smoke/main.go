// M0 §4b Go SDK smoke — the language-decision check (all Temporal code is Go).
//
// NOTE: the M0 install doc specifies the behavior ("a minimal Go program, one
// greet workflow + worker ... expect: 'hello from greet'") but does not give
// the full source; this minimal implementation is a fill-in written to that
// contract (install.sh greps the output for "hello from greet").
//
// Usage: go run . -addr vm-xb:7233   (worker + one workflow, prints result)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
)

const taskQueue = "go-sdk-smoke"

// GreetWorkflow is the trivial workflow the smoke run executes.
func GreetWorkflow(ctx context.Context, name string) (string, error) {
	return "hello from greet (" + name + ")", nil
}

func main() {
	addr := flag.String("addr", "127.0.0.1:7233", "Temporal frontend address (host:port)")
	flag.Parse()

	c, err := client.Dial(client.Options{HostPort: *addr, Namespace: "default"})
	if err != nil {
		log.Fatalf("dial %s: %v", *addr, err)
	}
	defer c.Close()

	// Run the worker in-process for this smoke run.
	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterWorkflow(GreetWorkflow)
	if err := w.Start(); err != nil {
		log.Fatalf("worker start: %v", err)
	}
	defer w.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	opts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("go-sdk-smoke-%d", time.Now().UnixNano()),
		TaskQueue: taskQueue,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}
	run, err := c.ExecuteWorkflow(ctx, opts, GreetWorkflow, "smoke")
	if err != nil {
		log.Fatalf("start workflow: %v", err)
	}
	var out string
	if err := run.Get(ctx, &out); err != nil {
		log.Fatalf("workflow: %v", err)
	}
	fmt.Println(out) // expect: hello from greet (smoke)
}
