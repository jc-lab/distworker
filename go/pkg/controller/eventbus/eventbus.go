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

package eventbus

import (
	"context"
	"sync"
)

type Bus[T interface{}] interface {
	Publish(id string, data T)
	Listen(ctx context.Context, id string) (T, error)
}

type eventTopic[T interface{}] struct {
	done chan struct{}
	data T
}

type busImpl[T interface{}] struct {
	mu     sync.Mutex
	topics map[string]*eventTopic[T]
}

var _ Bus[any] = (*busImpl[any])(nil)

func New[T interface{}]() Bus[T] {
	return &busImpl[T]{
		topics: make(map[string]*eventTopic[T]),
	}
}

func (b *busImpl[T]) Publish(id string, data T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic := b.topics[id]
	if topic != nil {
		topic.data = data
		delete(b.topics, id)
		close(topic.done)
	}
}

func (b *busImpl[T]) Listen(ctx context.Context, id string) (T, error) {
	topic := b.getOrCreateTopic(id)
	return topic.waitForData(ctx)
}

func newEventTopic[T interface{}]() *eventTopic[T] {
	topic := &eventTopic[T]{
		done: make(chan struct{}),
	}
	return topic
}

func (t *eventTopic[T]) waitForData(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		var empty T
		return empty, ctx.Err()
	case _, _ = <-t.done:
		return t.data, nil
	}
}

func (b *busImpl[T]) getOrCreateTopic(id string) *eventTopic[T] {
	b.mu.Lock()
	defer b.mu.Unlock()

	if topic, exists := b.topics[id]; exists {
		return topic
	}

	topic := newEventTopic[T]()
	b.topics[id] = topic
	return topic
}
