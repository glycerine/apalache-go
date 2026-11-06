package base

// Source: src/muz/base/dl_engine_base.h engine_base::cleanup.

func (e *EngineBase) Cleanup() error {
	if x := e.open(); x != nil {
		return x
	}
	if capability, ok := e.core.(EngineCleaner); ok {
		return capability.Cleanup()
	}
	return nil
}
