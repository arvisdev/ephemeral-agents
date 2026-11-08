// Copyright 2026 A&R Visual Development Inc (arvisdev)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package publisher publishes task responses to the agent.tasks.response
// queue via the RabbitMQ DEFAULT exchange (empty exchange name, queue name as
// the routing key). The queue is declared unbound by M0 §3 by design.
package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// QueueName is the response queue; also the routing key on the default exchange.
const QueueName = "agent.tasks.response"

// AMQP wraps a lazily-reconnecting connection+channel.
type AMQP struct {
	url  string
	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func New(url string) *AMQP { return &AMQP{url: url} }

func (p *AMQP) channel() (*amqp.Channel, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil && p.conn != nil && !p.conn.IsClosed() {
		return p.ch, nil
	}
	p.ch, p.conn = nil, nil
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	// Declare idempotently (M0 owns topology; this keeps publish self-healing).
	// durable=true matches M0 §3's declaration — a durability mismatch would
	// make this re-declare fail with a PRECONDITION_FAILED channel error.
	if _, err := ch.QueueDeclare(QueueName, true, false, false, false, nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	p.conn, p.ch = conn, ch
	return ch, nil
}

// PublishResponse sends the correlated response on the default exchange.
func (p *AMQP) PublishResponse(ctx context.Context, r taskconfig.Response) error {
	body, err := marshal(r)
	if err != nil {
		return err
	}
	ch, err := p.channel()
	if err != nil {
		return err
	}
	pctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return ch.PublishWithContext(pctx, "" /* default exchange */, QueueName, false, false,
		amqp.Publishing{ContentType: "application/json", Body: body})
}

func marshal(r taskconfig.Response) ([]byte, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return nil, err
	}
	return b, nil
}
