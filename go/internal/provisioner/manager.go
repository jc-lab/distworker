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
	"fmt"
	"github.com/jc-lab/distworker/go/internal/worker"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"go.uber.org/zap"
)

type Manager interface {
	SetWorkerManager(workerManager worker.Manager)

	GetDatabase() database.Database
	GetWorkerManager() worker.Manager
	GetWorkerAccessibleBaseUrl() string
	GetProvisioner(name string) Provisioner
	GetProvisioners() []Provisioner
}

type managerImpl struct {
	Provisioners map[string]Provisioner

	rootCtx      context.Context
	workerConfig *config.WorkerConfig
	db           database.Database
	wm           worker.Manager
}

func (m *managerImpl) GetDatabase() database.Database {
	return m.db
}

func (m *managerImpl) GetWorkerManager() worker.Manager {
	return m.wm
}

func (m *managerImpl) GetWorkerAccessibleBaseUrl() string {
	return m.workerConfig.AccessibleBaseUrl
}

func NewManager(
	rootCtx context.Context,
	cfg config.ProvisionerConfig,
	workerConfig *config.WorkerConfig,
	db database.Database,
	logger *zap.Logger,
) (Manager, error) {
	manager := &managerImpl{
		rootCtx:      rootCtx,
		Provisioners: make(map[string]Provisioner),
		workerConfig: workerConfig,
		db:           db,
	}

	for name, settings := range cfg {
		switch settings.Type {
		case config.ProvisionerTypeNone:
			manager.Provisioners[name] = &NoneProvisioner{
				Base: Base{
					Name:                name,
					ProvisionerSettings: settings,
					rootCtx:             rootCtx,
					manager:             manager,
					logger:              logger.Named(name).Sugar(),
				},
			}
		case config.ProvisionerTypeCommand:
			manager.Provisioners[name] = &CommandProvisioner{
				Base: Base{
					Name:                name,
					rootCtx:             rootCtx,
					ProvisionerSettings: settings,
					manager:             manager,
					logger:              logger.Named(name).Sugar(),
				},
			}
		default:
			return nil, fmt.Errorf("unknown provisioner type: %s", settings.Type)
		}
	}

	return manager, nil
}

func (m *managerImpl) SetWorkerManager(wm worker.Manager) {
	m.wm = wm
}

func (m *managerImpl) GetProvisioner(name string) Provisioner {
	return m.Provisioners[name]
}

func (m *managerImpl) GetProvisioners() []Provisioner {
	provisioners := make([]Provisioner, 0, len(m.Provisioners))
	for _, provisioner := range m.Provisioners {
		provisioners = append(provisioners, provisioner)
	}
	return provisioners
}
