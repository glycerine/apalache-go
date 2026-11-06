package shell

// Source: src/shell/smtlib_frontend.cpp display_statistics and total-time
// accounting around parser execution.

import "io"

func (f *SMTLIBFrontend) writeFinalStatistics(seconds float64) error {
	if f == nil || f.context == nil || f.output == nil {
		return ErrShellOptionService
	}
	if err := f.context.SetStringOption("regular-output-channel", "stdout"); err != nil {
		return err
	}
	return f.context.WriteStatisticsWithTotalTime(f.output, seconds)
}

// WriteStatistics exposes the source timeout/signal reporting boundary to
// embedders without installing process-global signal handlers. The caller
// decides when a report is appropriate; the frontend retains no goroutine.
func (f *SMTLIBFrontend) WriteStatistics(output io.Writer) error {
	if f == nil || f.closed || f.context == nil || output == nil {
		return ErrShellOptionService
	}
	seconds, err := f.watch.Seconds()
	if err != nil {
		return err
	}
	return f.context.WriteStatisticsWithTotalTime(output, seconds)
}
