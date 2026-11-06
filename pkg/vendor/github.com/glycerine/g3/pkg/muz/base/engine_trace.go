package base

// EngineTrace is the concrete retained rule-vector result of get_rules_along_trace.
type EngineTrace struct{ Rules []*Rule }

func (e *EngineBase) RulesAlongTrace() (*EngineTrace, error) {
	if x := e.open(); x != nil {
		return nil, x
	}
	if capability, ok := e.core.(EngineTraceProvider); ok {
		return capability.RulesAlongTrace()
	}
	return nil, e.unsupported("get_rules_along_trace")
}
