package api

// Source: src/shell/z3_log_frontend.cpp, solve. The shell dispatcher and
// generated API-command registration are separate port steps.

import (
	"errors"
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// ReplayZ3LogStream runs an already configured concrete replayer and emits
// the source error, peak-memory and CPU-time reports. The caller owns the
// replayer and its input; registered API commands remain on that replayer.
func ReplayZ3LogStream(replayer *Z3Replayer, output, diagnostics io.Writer, process *memory.ProcessAccounting, clock func() (uint64, error)) error {
	if replayer == nil || output == nil || diagnostics == nil || process == nil || !process.Valid() {
		return ErrZ3ReplayState
	}
	watch := stopwatch.New(clock)
	if err := watch.Start(); err != nil {
		return err
	}
	parseErr := replayer.Parse()
	if parseErr != nil {
		var replayErr *Z3ReplayError
		if !errors.As(parseErr, &replayErr) {
			return parseErr
		}
		if _, err := fmt.Fprintf(diagnostics, "Error at line %d: %s\n", replayer.Line(), replayErr.Message); err != nil {
			return err
		}
	}
	if err := watch.Stop(); err != nil {
		return err
	}
	seconds, err := watch.Seconds()
	if err != nil {
		return err
	}
	if err = process.DisplayMaxUsage(output); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "time:               %.6g\n", seconds)
	return err
}
