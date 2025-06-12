// distworker
// Copyright (C) 2025 JC-Lab
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package subscriber

import (
	"context"
	"sync"
)

type Listener[T interface{}, P interface{}] interface {
	Progress(ctx context.Context) chan P
	// Wait for finish
	Wait(ctx context.Context) (T, error)
}

type Bus[T interface{}, P interface{}] interface {
	EmitProgress(id string, data P) bool
	Finish(id string, data T) bool
	Listener(id string) (Listener[T, P], error)
}

type eventTopic[T interface{}, P interface{}] struct {
	mu       sync.Mutex
	cond     *sync.Cond
	doneCh   chan struct{}
	done     bool
	progress []P
	data     T
}

type busImpl[T interface{}, P interface{}] struct {
	mu     sync.Mutex
	topics map[string]*eventTopic[T, P]
}

var _ Bus[any, any] = (*busImpl[any, any])(nil)

func New[T interface{}, P interface{}]() Bus[T, P] {
	return &busImpl[T, P]{
		topics: make(map[string]*eventTopic[T, P]),
	}
}

func (b *busImpl[T, P]) EmitProgress(id string, data P) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic := b.topics[id]
	if topic != nil {
		topic.progress = append(topic.progress, data)
		topic.cond.Broadcast()
	}
	return topic != nil
}

func (b *busImpl[T, P]) Finish(id string, data T) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic := b.topics[id]
	if topic != nil {
		topic.done = true
		topic.data = data
		delete(b.topics, id)
		close(topic.doneCh)
		topic.cond.Broadcast()
	}
	return topic != nil
}

func (b *busImpl[T, P]) Listener(id string) (Listener[T, P], error) {
	return b.getOrCreateTopic(id), nil
}

func newEventTopic[T interface{}, P interface{}]() *eventTopic[T, P] {
	topic := &eventTopic[T, P]{
		doneCh: make(chan struct{}),
	}
	topic.cond = sync.NewCond(&notLocker{})
	return topic
}

func (t *eventTopic[T, P]) Wait(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		var empty T
		return empty, ctx.Err()
	case _, _ = <-t.doneCh:
		return t.data, nil
	}
}

func (t *eventTopic[T, P]) Progress(ctx context.Context) chan P {
	ch := make(chan P)
	go func() {
		defer close(ch)

		progressIndex := 0
		for ctx.Err() == nil && !t.done {
			progressSize := len(t.progress)
			for progressIndex < progressSize {
				ch <- t.progress[progressIndex]
				progressIndex++
			}
			t.cond.Wait()
		}
	}()
	return ch
}

func (b *busImpl[T, P]) getOrCreateTopic(id string) *eventTopic[T, P] {
	b.mu.Lock()
	defer b.mu.Unlock()

	if topic, exists := b.topics[id]; exists {
		return topic
	}

	topic := newEventTopic[T, P]()
	b.topics[id] = topic
	return topic
}
