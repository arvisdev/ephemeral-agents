// Command adapter is the M1 RabbitMQ -> Temporal topic bridge.
//
// Correctness contract (M1 installation doc §1):
//  1. QoS(prefetch=1), manual ack.
//  2. Consume the durable queue bound to the agent.tasks topic exchange.
//  3. Validate each message against schemas/message-v1.json; invalid ->
//     publish to the DLQ with the failure reason, then ack (never requeue).
//  4. Valid -> client.StartWorkflow with WorkflowID = task_id and
//     WorkflowIDReusePolicy=AllowDuplicate; on ErrWorkflowExecutionAlreadyExists
//     ack (idempotent duplicate); otherwise ack only after start is confirmed.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
)

// Env var names per the M1 compose block.
const (
	envRabbitMQURL   = "RABBITMQ_URL"   // e.g. amqp://poc:***@rabbitmq:5672/
	envTemporalAddr  = "TEMPORAL_ADDR"  // e.g. temporal:7233
	envTemporalNS    = "TEMPORAL_NAMESPACE" // optional, default "default"
	envTasksExchange = "TASKS_EXCHANGE" // agent.tasks
	envTasksQueue    = "TASKS_QUEUE"    // agent.tasks.durable
	envDLQQueue      = "DLQ_QUEUE"      // agent.tasks.dlq
)

// TaskWorkflow is the workflow started per valid task message (M1 doc §1 step 4).
const TaskWorkflow = "TaskWorkflow"

// DLQ reason header/property key for rejected messages.
const dlqReasonHeader = "x-reject-reason"

// TaskMessage mirrors schemas/message-v1.json.
// additionalProperties=false and required-field checks live in ValidateTaskMessage.
type TaskMessage struct {
	TaskID         string `json:"task_id"`
	AgentRef       string `json:"agent_ref"`
	BehavioralSpec string `json:"behavioral_spec"`
	RepoTarget     string `json:"repo_target"`
	Requester      string `json:"requester"`
	TTLS           *int64 `json:"ttl_s,omitempty"`
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

var requiredStringFields = []struct {
	name string
}{
	{"task_id"}, {"agent_ref"}, {"behavioral_spec"}, {"repo_target"}, {"requester"},
}

// allowedFields mirrors the schema's `properties`; anything else is rejected
// because the schema says additionalProperties: false.
var allowedFields = map[string]bool{
	"task_id": true, "agent_ref": true, "behavioral_spec": true,
	"repo_target": true, "requester": true, "ttl_s": true,
}

// ValidateTaskMessage validates a raw message body against schemas/message-v1.json
// exactly: required task_id (UUID format), agent_ref, behavioral_spec, repo_target,
// requester; optional ttl_s integer >= 1; additionalProperties=false.
// Returns "" when valid, otherwise the failure reason.
func ValidateTaskMessage(body []byte) string {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Sprintf("invalid JSON: %v", err)
	}

	for k := range raw {
		if !allowedFields[k] {
			return fmt.Sprintf("additional property %q not allowed (additionalProperties=false)", k)
		}
	}

	var m TaskMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return fmt.Sprintf("invalid message shape: %v", err)
	}

	for _, f := range requiredStringFields {
		v, ok := raw[f.name]
		if !ok || len(v) == 0 || string(v) == "null" {
			return fmt.Sprintf("missing required field %q", f.name)
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return fmt.Sprintf("field %q must be a string", f.name)
		}
	}

	if !uuidRe.MatchString(m.TaskID) {
		return fmt.Sprintf("task_id %q is not a valid UUID", m.TaskID)
	}

	if v, ok := raw["ttl_s"]; ok {
		var n json.Number
		if err := json.Unmarshal(v, &n); err != nil {
			return "ttl_s must be an integer"
		}
		ttl, err := n.Int64()
		if err != nil {
			return "ttl_s must be an integer"
		}
		if ttl < 1 {
			return fmt.Sprintf("ttl_s=%d is below the schema minimum of 1", ttl)
		}
	}

	return ""
}

// dlqPublish publishes the raw body to the DLQ with the rejection reason as a
// message header. Returns an error if the DLQ publish itself fails, in which
// case the caller must NOT ack (the message will be redelivered).
func dlqPublish(ch *amqp.Channel, dlq, reason string, body []byte) error {
	return ch.PublishWithContext(context.Background(),
		"", // default exchange -> direct publish to queue by name
		dlq,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Headers:      amqp.Table{dlqReasonHeader: reason},
			Body:         body,
		})
}

// startWorkflow starts TaskWorkflow with WorkflowID = task_id.
// Returns (ackedAsDuplicate, error). Ok means the message may be acked.
func startWorkflow(ctx context.Context, tc client.Client, m TaskMessage) error {
	_, err := tc.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:                       m.TaskID,
		TaskQueue:                "poc-tasks", // matches M2's worker task queue
		WorkflowIDReusePolicy:    enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE,
		WorkflowExecutionTimeout: 24 * time.Hour,
	}, TaskWorkflow, m)
	return err
}

func handleMessage(ctx context.Context, ch *amqp.Channel, tc client.Client, dlq string, delivery amqp.Delivery) {
	// 3. Validate; invalid -> DLQ with reason, then ack (never requeue).
	if reason := ValidateTaskMessage(delivery.Body); reason != "" {
		log.Printf("invalid message (delivery tag %d): %s", delivery.DeliveryTag, reason)
		if err := dlqPublish(ch, dlq, reason, delivery.Body); err != nil {
			log.Printf("DLQ publish failed (delivery tag %d kept unacked for redelivery): %v", delivery.DeliveryTag, err)
			return // no ack: broker redelivers
		}
		if err := delivery.Ack(false); err != nil {
			log.Printf("ack failed after DLQ publish: %v", err)
		}
		return
	}

	var m TaskMessage
	if err := json.Unmarshal(delivery.Body, &m); err != nil {
		// Should be unreachable after validation; treat as a poison pill.
		log.Printf("unmarshal after valid schema check failed: %v", err)
		if err := dlqPublish(ch, dlq, "post-validation unmarshal failed: "+err.Error(), delivery.Body); err != nil {
			return
		}
		_ = delivery.Ack(false)
		return
	}

	// 4. Valid: start the workflow, ack only after the start is confirmed.
	err := startWorkflow(ctx, tc, m)
	if temporal.IsWorkflowExecutionAlreadyStartedError(err) {
		// Idempotent duplicate: ack as success.
		log.Printf("workflow already exists for task_id %s (duplicate) - acking", m.TaskID)
		if err := delivery.Ack(false); err != nil {
			log.Printf("ack failed (duplicate): %v", err)
		}
		return
	}
	if err != nil {
		// Transient failure: do NOT ack, broker redelivers; idempotency key
		// (workflow ID = task_id) makes the retry safe.
		log.Printf("StartWorkflow failed for task_id %s (left unacked): %v", m.TaskID, err)
		return
	}

	if err := delivery.Ack(false); err != nil {
		log.Printf("ack failed after workflow start for task_id %s: %v", m.TaskID, err)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	exchange := envOr(envTasksExchange, "agent.tasks")
	queue := envOr(envTasksQueue, "agent.tasks.durable")
	dlq := envOr(envDLQQueue, "agent.tasks.dlq")
	rabbitURL := os.Getenv(envRabbitMQURL)
	temporalAddr := envOr(envTemporalAddr, "localhost:7233")
	namespace := envOr(envTemporalNS, "default")

	if rabbitURL == "" {
		log.Fatalf("%s is required", envRabbitMQURL)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Temporal client: reconnect is handled by the SDK; fatal only on first dial.
	tc, err := client.Dial(client.Options{
		HostPort:  temporalAddr,
		Namespace: namespace,
	})
	if err != nil {
		log.Fatalf("temporal dial %s (namespace %s): %v", temporalAddr, namespace, err)
	}
	defer tc.Close()

	for { // restart: unless-stopped wrapper; reconnect to RabbitMQ forever
		if err := consumeLoop(ctx, rabbitURL, exchange, queue, dlq, tc); err != nil {
			log.Printf("consume loop ended: %v; reconnecting in 5s", err)
		}
		select {
		case <-ctx.Done():
			log.Println("shutdown")
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func consumeLoop(ctx context.Context, rabbitURL, exchange, queue, dlq string, tc client.Client) error {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		return fmt.Errorf("rabbitmq dial: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("channel: %w", err)
	}
	defer ch.Close()

	// 1. QoS(prefetch=1), manual ack.
	if err := ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("qos(1): %w", err)
	}

	// Durable queue; DLQ bound to the same exchange's dead routing key.
	if _, err := ch.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue declare %s: %w", queue, err)
	}
	if _, err := ch.QueueDeclare(dlq, true, false, false, false, nil); err != nil {
		return fmt.Errorf("queue declare %s: %w", dlq, err)
	}
	routingKey := queue // M0 binds agent.tasks.durable to agent.tasks with the queue name as key
	if err := ch.QueueBind(queue, routingKey, exchange, false, nil); err != nil {
		return fmt.Errorf("bind %s -> %s: %w", queue, exchange, err)
	}
	if err := ch.QueueBind(dlq, dlq, exchange, false, nil); err != nil {
		return fmt.Errorf("bind %s -> %s: %w", dlq, exchange, err)
	}

	deliveries, err := ch.Consume(queue, "topic-bridge", false /* manual ack */, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("consume %s: %w", queue, err)
	}

	log.Printf("connected; consuming %s (exchange %s, prefetch=1, manual ack), DLQ %s", queue, exchange, dlq)

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed (connection lost)")
			}
			handleMessage(ctx, ch, tc, dlq, d)
		}
	}
}
