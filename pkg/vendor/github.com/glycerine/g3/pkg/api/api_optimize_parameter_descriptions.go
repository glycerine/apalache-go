package api

// Source: src/api/api_opt.cpp:191-202.

func (o *APIOptimize) ParameterDescriptions() (*ParameterDescriptions, error) {
	if err := o.open(); err != nil {
		return nil, err
	}
	context := &SMTContext{smtContextState: o.owner}
	return context.ParameterDescriptions("opt")
}
