package cmd_context

// Source: src/cmd_context/cmd_context.cpp:775-778,1860-1877;
// src/cmd_context/basic_cmds.cpp get_info_cmd :reason-unknown branch.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/escaped"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (c *Context) ReasonUnknown() (string, error) {
	if err := c.valid(); err != nil {
		return "", err
	}
	if !c.checked {
		return "state of the most recent check-sat command is not known", nil
	}
	return c.checkSatResult().ReasonUnknown()
}

// WriteStatistics publishes the active result's solver statistics, following
// the result-before-solver ordering in cmd_context::display_statistics.
func (c *Context) WriteStatistics(out io.Writer) error {
	return c.writeStatistics(out, nil)
}

// WriteStatisticsWithTotalTime publishes final shell statistics and includes
// the whole SMT-LIB frontend duration.
func (c *Context) WriteStatisticsWithTotalTime(out io.Writer, totalSeconds float64) error {
	return c.writeStatistics(out, &totalSeconds)
}

func (c *Context) writeStatistics(out io.Writer, totalSeconds *float64) error {
	if err := c.valid(); err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	st := statistics.New()
	if totalSeconds != nil {
		st.UpdateDouble("total time", *totalSeconds)
	}
	seconds, err := c.watch.Seconds()
	if err != nil {
		return err
	}
	st.UpdateDouble("time", seconds)
	{
		// Native memory counters are sampled before m().limit() initializes
		// a still-lazy manager. Keep that ordering for a first info command.
		runtime := c.config.Params.NumeralsField.RuntimeOwner()
		maximum, err := runtime.MaximumUsed()
		if err != nil {
			return err
		}
		used, err := runtime.AllocationSize()
		if err != nil {
			return err
		}
		allocations, err := runtime.AllocationCount()
		if err != nil {
			return err
		}
		st.UpdateDouble("max memory", float64((100*maximum)/(1024*1024))/100)
		st.UpdateDouble("memory", float64((100*used)/(1024*1024))/100)
		st.UpdateUint64("num allocs", allocations)
	}
	m, err := c.Manager()
	if err != nil {
		return err
	}
	st.UpdateUint64("rlimit count", m.Limit.Count())
	if result := c.checkSatResult(); result != nil {
		if err := result.CollectStatistics(st); err != nil {
			return err
		}
	}
	return st.DisplaySMT2(out)
}
func (c *Context) WriteReasonUnknown(out io.Writer) error {
	reason, err := c.ReasonUnknown()
	if err != nil {
		return err
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	if _, err = io.WriteString(out, "(:reason-unknown \""); err != nil {
		return err
	}
	if err = escaped.Write(out, &reason, false, 0); err != nil {
		return err
	}
	_, err = io.WriteString(out, "\")\n")
	return err
}
