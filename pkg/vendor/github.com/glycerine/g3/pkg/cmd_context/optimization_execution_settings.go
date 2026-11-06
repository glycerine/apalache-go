package cmd_context

// Source: src/api/api_opt.cpp:126-151, Z3_optimize_check local timeout and
// rlimit selection.

type optimizationExecutionSettings struct {
	timeout uint32
	rlimit  uint32
}

func (c *Context) optimizationSettings() (optimizationExecutionSettings, error) {
	state, err := c.parameters.State()
	if err != nil {
		return optimizationExecutionSettings{}, err
	}
	timeoutName, rlimitName := "timeout", "rlimit"
	timeout, err := c.config.Params.GetUIntName(&c.params, &timeoutName, state.Timeout)
	if err != nil {
		return optimizationExecutionSettings{}, err
	}
	rlimit, err := c.config.Params.GetUIntName(&c.params, &rlimitName, state.RLimit)
	if err != nil {
		return optimizationExecutionSettings{}, err
	}
	return optimizationExecutionSettings{timeout: timeout, rlimit: rlimit}, nil
}
