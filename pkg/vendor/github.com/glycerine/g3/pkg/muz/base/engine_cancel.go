package base

// Source: src/muz/base/dl_engine_base.h engine_base::cancel.

func (e *EngineBase) Cancel() error {
	if x := e.open(); x != nil {
		return x
	}
	if capability, ok := e.core.(EngineCanceler); ok {
		return capability.Cancel()
	}
	return nil
}
