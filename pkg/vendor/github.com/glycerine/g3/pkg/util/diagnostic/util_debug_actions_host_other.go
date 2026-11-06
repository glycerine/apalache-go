//go:build !linux

package diagnostic

// Source: src/util/debug.cpp:72-118; src/util/debug.h:42-56.

const debugHostLinux = false

func (a *DebugActionContext) hostRunCommand(string) (int, error) { return 0, ErrDebugHostUnsupported }
func (a *DebugActionContext) hostStop() error                    { return ErrDebugHostUnsupported }
