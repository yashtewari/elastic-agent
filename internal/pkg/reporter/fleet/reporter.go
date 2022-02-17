// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package fleet

import (
	"context"
	"sync"
	"time"

	"github.com/elastic/elastic-agent/internal/pkg/core/logger"
	"github.com/elastic/elastic-agent/internal/pkg/fleetapi"
	"github.com/elastic/elastic-agent/internal/pkg/reporter"
	"github.com/elastic/elastic-agent/internal/pkg/reporter/fleet/config"
)

type event struct {
	AgentID   string                 `json:"agent_id"`
	EventType string                 `json:"type"`
	Ts        fleetapi.Time          `json:"timestamp"`
	SubType   string                 `json:"subtype"`
	Msg       string                 `json:"message"`
	Payload   map[string]interface{} `json:"payload,omitempty"`
}

func (e *event) Type() string {
	return e.EventType
}

func (e *event) Timestamp() time.Time {
	return time.Time(e.Ts)
}

func (e *event) Message() string {
	return e.Msg
}

// Reporter is a reporter without any effects, serves just as a showcase for further implementations.
type Reporter struct {
	info      agentInfo
	logger    *logger.Logger
	queue     []fleetapi.SerializableEvent
	qlock     sync.Mutex
	threshold int
	lastAck   time.Time
}

type agentInfo interface {
	AgentID() string
}

// NewReporter creates a new fleet reporter.
func NewReporter(agentInfo agentInfo, l *logger.Logger, c *config.Config) (*Reporter, error) {
	r := &Reporter{
		info:      agentInfo,
		queue:     make([]fleetapi.SerializableEvent, 0),
		logger:    l,
		threshold: c.Threshold,
	}

	return r, nil
}

// Report enqueue event into reporter queue.
func (r *Reporter) Report(ctx context.Context, e reporter.Event) error {
	r.qlock.Lock()
	defer r.qlock.Unlock()

	r.queue = append(r.queue, &event{
		AgentID:   r.info.AgentID(),
		EventType: e.Type(),
		Ts:        fleetapi.Time(e.Time()),
		SubType:   e.SubType(),
		Msg:       e.Message(),
		Payload:   e.Payload(),
	})

	if r.threshold > 0 && len(r.queue) > r.threshold {
		// drop some low importance event if needed
		r.dropEvent()
	}

	return nil
}

// Events returns a list of event from a queue and a ack function
// which clears those events once caller is done with processing.
func (r *Reporter) Events() ([]fleetapi.SerializableEvent, func()) {
	r.qlock.Lock()
	defer r.qlock.Unlock()

	cp := r.queueCopy()

	ackFn := func() {
		// as time is monotonic and this is on single machine this should be ok.
		r.clear(cp, time.Now())
	}

	return cp, ackFn
}

func (r *Reporter) clear(items []fleetapi.SerializableEvent, ackTime time.Time) {
	r.qlock.Lock()
	defer r.qlock.Unlock()

	if ackTime.Sub(r.lastAck) <= 0 ||
		len(r.queue) == 0 ||
		items == nil ||
		len(items) == 0 {
		return
	}

	var dropIdx int
	r.lastAck = ackTime
	itemsLen := len(items)

OUTER:
	for idx := itemsLen - 1; idx >= 0; idx-- {
		for i, v := range r.queue {
			if v == items[idx] {
				dropIdx = i
				break OUTER
			}
		}
	}

	r.queue = r.queue[dropIdx+1:]
}

// Close stops all the background jobs reporter is running.
// Guards agains panic of closing channel multiple times.
func (r *Reporter) Close() error {
	return nil
}

func (r *Reporter) queueCopy() []fleetapi.SerializableEvent {
	size := len(r.queue)
	batch := make([]fleetapi.SerializableEvent, size)

	copy(batch, r.queue)
	return batch
}

func (r *Reporter) dropEvent() {
	if dropped := r.tryDropInfo(); !dropped {
		r.dropFirst()
	}
}

// tryDropInfo returns true if info was found and dropped.
func (r *Reporter) tryDropInfo() bool {
	for i, e := range r.queue {
		if e.Type() != reporter.EventTypeError {
			r.queue = append(r.queue[:i], r.queue[i+1:]...)
			r.logger.Infof("fleet reporter dropped event because threshold[%d] was reached: %v", r.threshold, e)
			return true
		}
	}

	return false
}

func (r *Reporter) dropFirst() {
	if len(r.queue) == 0 {
		return
	}

	first := r.queue[0]
	r.logger.Infof("fleet reporter dropped event because threshold[%d] was reached: %v", r.threshold, first)
	r.queue = r.queue[1:]
}

// Check it is reporter.Backend.
var _ reporter.Backend = &Reporter{}