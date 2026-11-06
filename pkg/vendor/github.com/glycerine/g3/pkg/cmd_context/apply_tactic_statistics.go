package cmd_context

// Source: src/cmd_context/tactic_cmds.cpp:151-158, display_statistics.

import (
	"io"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func (c *Context) writeAppliedTacticStatistics(out io.Writer, selected tactic.Tactic) error {
	values := statistics.New()
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
	values.UpdateDouble("max memory", float64((100*maximum)/(1024*1024))/100)
	values.UpdateDouble("memory", float64((100*used)/(1024*1024))/100)
	values.UpdateUint64("num allocs", allocations)
	statistics.AddResourceLimit(values, c.manager.ResourceLimit().Count())
	seconds, err := c.watch.Seconds()
	if err != nil {
		return err
	}
	values.UpdateDouble("time", seconds)
	for name, value := range selected.CollectStatistics() {
		values.UpdateDouble(statistics.Key(name), value)
	}
	return values.DisplaySMT2(out)
}
