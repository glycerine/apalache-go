package memory

// Source: src/util/util.h:181; src/util/small_object_allocator.cpp:172-242.
// One borrowed cold dispatcher. Byte formatting remains in the caller package.

import (
	"errors"
)

type AllocatorIOEvent uint8

const (
	AllocatorIOLeak AllocatorIOEvent = iota + 1
	AllocatorIOVerboseStart
	AllocatorIOVerboseEnd
)

// AllocatorVerboseControl is a borrowed cold source-level getter.
type AllocatorVerboseControl interface{ VerbosityLevel() (uint32, error) }

type AllocatorIO interface {
	Emit(*SmallAllocator, AllocatorIOEvent) error
}

var ErrAllocatorIO = errors.New("memory: missing allocator diagnostic transport")

func (a *SmallAllocator) emit(event AllocatorIOEvent) error {
	if a.config.IO == nil {
		return ErrAllocatorIO
	}
	return a.config.IO.Emit(a, event)
}
func (a *SmallAllocator) diagnostic() error {
	if !a.config.Profile.Debug || a.allocSize == 0 {
		return nil
	}
	return a.emit(AllocatorIOLeak)
}
func (a *SmallAllocator) verbose(end bool) error {
	level := a.config.Verbosity
	if a.config.VerboseControl != nil {
		var e error
		level, e = a.config.VerboseControl.VerbosityLevel()
		if e != nil {
			return e
		}
	}
	if level < 20 {
		return nil
	}
	if end {
		return a.emit(AllocatorIOVerboseEnd)
	}
	return a.emit(AllocatorIOVerboseStart)
}
