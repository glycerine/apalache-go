package base

func (e *EngineBase) Close() error {
	if e == nil || e.manager == nil {
		return ErrEngineBase
	}
	if e.closed {
		return nil
	}
	e.core = nil
	e.closed = true
	return nil
}
