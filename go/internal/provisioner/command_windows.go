//test

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
