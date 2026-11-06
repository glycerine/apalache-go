package base

// Source: src/muz/base/dl_engine_base.h engine_base::updt_params.

func (e *EngineBase) UpdateParameters() error {
	if x := e.open(); x != nil {
		return x
	}
	if capability, ok := e.core.(EngineParameterUpdater); ok {
		return capability.UpdateParameters()
	}
	return nil
}
