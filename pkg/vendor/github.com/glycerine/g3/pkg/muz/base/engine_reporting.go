package base

import (
	"github.com/glycerine/g3/pkg/util/statistics"
	"io"
)

func (e *EngineBase) ResetStatistics() error {
	if x := e.open(); x != nil {
		return x
	}
	if capability, ok := e.core.(EngineStatistics); ok {
		return capability.ResetStatistics()
	}
	return nil
}

func (e *EngineBase) CollectStatistics(destination *statistics.Statistics) error {
	if x := e.open(); x != nil {
		return x
	}
	if destination == nil {
		return ErrEngineBase
	}
	if capability, ok := e.core.(EngineStatistics); ok {
		return capability.CollectStatistics(destination)
	}
	return nil
}

func (e *EngineBase) DisplayProfile(out io.Writer) error {
	if x := e.open(); x != nil {
		return x
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if capability, ok := e.core.(EngineStatistics); ok {
		return capability.DisplayProfile(out)
	}
	return nil
}
