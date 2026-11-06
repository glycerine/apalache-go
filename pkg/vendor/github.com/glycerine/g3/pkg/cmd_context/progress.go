package cmd_context

// Source: src/cmd_context/cmd_context.cpp set_cancel and progress callbacks.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/statistics"
)

// SetCancel updates the live manager's resource-limit cancellation state.
// Like the source method, it does not initialize a manager on a cold context.
func (c *Context) SetCancel(cancel bool) error {
	if err := c.valid(); err != nil {
		return err
	}
	if c.manager == nil {
		return nil
	}
	limit := c.manager.ResourceLimit()
	if cancel {
		limit.Cancel()
	} else {
		limit.ResetCancel()
	}
	return nil
}

// FastProgressSample is intentionally empty in cmd_context.cpp.
func (c *Context) FastProgressSample() {}

// SlowProgressSample prints the current solver statistics and labels in the
// source progress form. Writer errors are returned by the enclosing CheckSat.
func (c *Context) SlowProgressSample() {
	if c == nil || c.solver == nil || c.closed {
		return
	}
	if c.progressError != nil {
		return
	}
	out := c.RegularOutput()
	if c.progressError = writeProgressHeader(out); c.progressError != nil {
		return
	}
	var stats statistics.Statistics
	if err := c.solver.CollectStatistics(&stats); err != nil {
		c.progressError = err
		return
	}
	if err := stats.DisplaySMT2(out); err != nil {
		c.progressError = err
		return
	}
	if _, err := io.WriteString(out, "(labels"); err != nil {
		c.progressError = err
		return
	}
	labels, err := c.solver.Labels()
	if err != nil {
		c.progressError = err
		return
	}
	for _, label := range labels {
		name, err := c.manager.Symbols.String(label)
		if err != nil {
			c.progressError = err
			return
		}
		if _, err = io.WriteString(out, " "+name); err != nil {
			c.progressError = err
			return
		}
	}
	if _, err := io.WriteString(out, "))\n"); err != nil {
		c.progressError = err
	}
}

func writeProgressHeader(out io.Writer) error {
	_, err := io.WriteString(out, "(progress\n")
	return err
}

var _ interface {
	FastProgressSample()
	SlowProgressSample()
} = (*Context)(nil)
