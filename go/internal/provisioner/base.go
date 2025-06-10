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

package provisioner

import (
	"context"
	"errors"
	"github.com/jc-lab/distworker/go/internal/worker"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"go.uber.org/zap"
	"sync/atomic"
	"time"
)

type Provisioner interface {
	GetName() string
	GetSetting() *config.ProvisionerSettings
	IsGrowable(taskDelay time.Duration) bool
	Grow(ctx context.Context) (worker.Connection, error)
}

type Base struct {
	Name string
	config.ProvisionerSettings

	rootCtx context.Context
	manager Manager
	logger  *zap.SugaredLogger

	CurrentWorkers atomic.Int32
}

func (p *Base) GetName() string {
	return p.Name
}

func (p *Base) GetSetting() *config.ProvisionerSettings {
	return &p.ProvisionerSettings
}

func (p *Base) IsGrowable(taskDelay time.Duration) bool {
	if p.Type == config.ProvisionerTypeNone {
		return false
	}
	return int(p.CurrentWorkers.Load()) < p.Autoscale.Max && (p.Autoscale.UpLatency <= taskDelay || p.CurrentWorkers.Load() == 0)
}

func (p *Base) Grow(ctx context.Context) (worker.Connection, error) {
	return nil, errors.New("not implemented")
}
