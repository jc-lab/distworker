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
	"github.com/google/uuid"
	"github.com/jc-lab/distworker/go/internal/mergedcontext"
	"github.com/jc-lab/distworker/go/internal/worker"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"github.com/pkg/errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CommandEnvMap map[string]string
type CommandSysPaths []string

type CommandProvisioner struct {
	Base
}

type commandInstance struct {
	db database.Database

	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd
}

func (i *commandInstance) Shutdown(timeout time.Duration) {
	_ = sendSignal(i.cmd.Process, os.Interrupt)
	select {
	case <-time.After(timeout):
		_ = i.cmd.Process.Kill()
	case <-i.ctx.Done():
	}
}

func (p *CommandProvisioner) Grow(ctx context.Context) (worker.Connection, error) {
	instance := &commandInstance{}
	instance.ctx, instance.cancel = context.WithCancel(context.Background())

	db := p.manager.GetDatabase()

	session := &models.WorkerSession{
		WorkerId:        p.GetName() + "-" + uuid.NewString(),
		Health:          types.WorkerHealthDisconnected,
		Queues:          p.ProvisionerSettings.Queues,
		ProvisionerName: p.GetName(),
		WorkerToken:     uuid.NewString(),
	}
	if err := db.GetWorkerSessionRepository().Create(ctx, session); err != nil {
		return nil, errors.Wrap(err, "unable to create worker session")
	}

	// startup timeout

	startedCh := make(chan struct{})
	defer close(startedCh)
	go func() {
		select {
		case <-ctx.Done():
			instance.cancel()
		case <-startedCh:
		}
	}()

	program := p.Command.Command[0]

	env := NewCommandEnvMap(os.Environ())
	if p.Command.Envs != nil {
		for k, v := range p.Command.Envs {
			env[k] = v
		}
	}
	if p.Command.Venv != "" {
		paths := GetOsPaths(env["PATH"])
		venvPath := filepath.Join(p.Command.Venv, "bin")
		paths = append([]string{venvPath}, paths...)
		env["PATH"] = paths.String()
		env["VIRTUAL_ENV"] = p.Command.Venv

		candinate := filepath.Join(venvPath, program)
		if _, err := os.Stat(candinate); err == nil {
			program = candinate
		}
	}
	env["DISTWORKER_CONTROLLER_URL"] = p.manager.GetWorkerAccessibleBaseUrl()
	env["DISTWORKER_PROVISIONER"] = session.ProvisionerName
	env["DISTWORKER_WORKER_ID"] = session.WorkerId
	env["DISTWORKER_WORKER_TOKEN"] = session.WorkerToken

	instance.cmd = exec.CommandContext(ctx, program, p.Command.Command[1:]...)
	if p.Command.WorkingDirectory != "" {
		instance.cmd.Dir = p.Command.WorkingDirectory
	}
	instance.cmd.Env = env.List()

	instance.cmd.Stderr = &logWriter{
		logger: p.logger,
		prefix: fmt.Sprintf("Worker[%s] stderr: ", session.WorkerId),
	}
	instance.cmd.Stdout = &logWriter{
		logger: p.logger,
		prefix: fmt.Sprintf("Worker[%s] stdout: ", session.WorkerId),
	}

	if err := instance.cmd.Start(); err != nil {
		instance.cancel()
		_ = db.GetWorkerSessionRepository().Delete(ctx, session.WorkerId)
		return nil, err
	}

	go func() {
		_ = instance.cmd.Wait()
		instance.cancel()
	}()
	go func() {
		select {
		case <-p.rootCtx.Done():
			// gracefully shutdown
			instance.Shutdown(time.Second * 5)
		case <-instance.ctx.Done():
			return
		}
	}()

	waitCtx, waitCancel := mergedcontext.WithMultiple(ctx, instance.ctx)
	defer waitCancel()

	conn, err := p.manager.GetWorkerManager().WaitForWorkerReady(waitCtx, session.WorkerId)
	if err != nil {
		_ = db.GetWorkerSessionRepository().Delete(ctx, session.WorkerId)
		go instance.Shutdown(time.Second * 5)
		return nil, err
	}

	return conn, nil
}

func NewCommandEnvMap(input []string) CommandEnvMap {
	output := make(CommandEnvMap)
	for _, s := range input {
		tokens := strings.SplitN(s, "=", 2)
		if len(tokens) == 2 {
			output[tokens[0]] = tokens[1]
		}
	}
	return output
}

func (e CommandEnvMap) List() []string {
	output := make([]string, len(e))
	for k, v := range e {
		output = append(output, k+"="+v)
	}
	return output
}
