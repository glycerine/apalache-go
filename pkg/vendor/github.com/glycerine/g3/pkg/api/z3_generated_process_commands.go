package api

// Source: generated src/api/api_commands.cpp, process and version commands.

import "github.com/glycerine/g3/pkg/util/process"
import "github.com/glycerine/g3/pkg/engine"

// RegisterZ3GeneratedProcessCommands installs the implemented process-wide
// warning, version and trace replay handlers.
func RegisterZ3GeneratedProcessCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{327, "Z3_toggle_warning_messages", replayToggleWarnings},
		{342, "Z3_get_version", replayGetVersion},
		{343, "Z3_get_full_version", replayGetFullVersion},
		{344, "Z3_enable_trace", replayEnableTrace},
		{345, "Z3_disable_trace", replayDisableTrace},
		{347, "Z3_finalize_memory", replayFinalizeMemory},
		{346, "Z3_reset_memory", replayResetMemory},
		{446, "Z3_get_estimated_alloc_size", replayEstimatedAllocSize},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayFinalizeMemory(*Z3Replayer) error { return process.FinalizeCurrentGenerated() }
func replayResetMemory(*Z3Replayer) error    { return engine.ResetDefaultProcess() }
func replayToggleWarnings(r *Z3Replayer) error {
	enabled, err := r.Bool(0)
	if err != nil {
		return err
	}
	ToggleWarningMessages(enabled)
	return nil
}
func replayGetVersion(r *Z3Replayer) error {
	for i := uint32(0); i < 4; i++ {
		if _, err := r.Uint(i); err != nil {
			return err
		}
	}
	version := Version()
	for i, value := range []uint32{version.Major, version.Minor, version.Build, version.Revision} {
		if err := r.SetUint64(uint32(i), uint64(value)); err != nil {
			return err
		}
	}
	return nil
}
func replayGetFullVersion(*Z3Replayer) error     { _ = FullVersion(); return nil }
func replayEstimatedAllocSize(*Z3Replayer) error { _ = EstimatedAllocSize(); return nil }
func replayEnableTrace(r *Z3Replayer) error {
	tag, err := r.String(0)
	if err != nil {
		return err
	}
	return EnableTrace(tag)
}
func replayDisableTrace(r *Z3Replayer) error {
	tag, err := r.String(0)
	if err != nil {
		return err
	}
	DisableTrace(tag)
	return nil
}
