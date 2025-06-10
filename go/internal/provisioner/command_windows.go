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
	"golang.org/x/sys/windows"
	"os"
	"strings"
	"syscall"
)

func GetOsPaths(input string) CommandSysPaths {
	if input == "" {
		input = os.Getenv("PATH")
	}
	return strings.Split(input, ";")
}

func (paths CommandSysPaths) String() string {
	return strings.Join(paths, ";")
}

func sendSignal(process *os.Process, signal os.Signal) error {
	if signal == os.Interrupt {
		return sendCtrlBreak(process.Pid)
	}
	return process.Signal(signal)
}

func sendCtrlBreak(pid int) error {
	return windows.GenerateConsoleCtrlEvent(syscall.CTRL_BREAK_EVENT, uint32(pid))
}
