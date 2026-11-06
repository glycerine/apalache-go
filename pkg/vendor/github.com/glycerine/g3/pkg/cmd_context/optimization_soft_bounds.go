package cmd_context

import "github.com/glycerine/g3/pkg/opt"

// publishOptimizationBounds copies available search bounds even when the
// search stops before finding an optimum. The result owns its values.
func (c *Context) publishOptimizationBounds(result *opt.ArithmeticResult) {
	for i := range c.objectives {
		objective := &c.objectives[i]
		objective.boundsReady = false
		if result == nil || i >= len(result.Values) {
			continue
		}
		value := &result.Values[i]
		if objective.soft && value.SoftBoundsValid {
			objective.boundLower.Set(&value.SoftLower)
			objective.boundUpper.Set(&value.SoftUpper)
		} else if !objective.soft && value.SourceBoundsValid {
			objective.boundLower.Set(&value.SourceLower)
			objective.boundUpper.Set(&value.SourceUpper)
		} else {
			continue
		}
		objective.boundsReady = true
	}
}
