//go:build linux

package diagnostic

// Source: src/util/debug.cpp:88-107.

import (
	"os/exec"
)

const debugHostLinux = true

func (a *DebugActionContext) hostRunCommand(command string) (int, error) {
	out, e := a.streams.Stdout()
	if e != nil {
		return 0, e
	}
	errout, e := a.streams.Stderr()
	if e != nil {
		return 0, e
	}
	if e = a.input.open(); e != nil {
		return 0, e
	}
	if e = out.CheckOpen(); e != nil {
		return 0, e
	}
	if e = errout.CheckOpen(); e != nil {
		return 0, e
	}
	// /bin/sh is the concrete system() adapter. The command is literal source
	// text; status reflects the actual child, with no injected result provider.
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdin = a.input.file
	cmd.Stdout = out.File
	cmd.Stderr = errout.File
	if e = cmd.Run(); e != nil {
		return 1, nil
	}
	return 0, nil
}
func (a *DebugActionContext) hostStop() error {
	// The source stops by writing through a null pointer. Sending SIGSEGV to
	// the process can land on another Go thread and leave this caller blocked.
	var stopped *int
	*stopped = 0
	return nil
}
