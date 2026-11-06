package api

// Source: src/api/api_opt.cpp:126-151, cancel_eh and
// api::context::set_interruptable around Z3_optimize_check.

import "errors"

type apiOptimizeInterruptScope struct {
	handler *smtContextCancelHandler
	scope   *smtContextInterruptScope
	closed  bool
}

func newAPIOptimizeInterruptScope(optimize *APIOptimize) (*apiOptimizeInterruptScope, error) {
	if err := optimize.open(); err != nil {
		return nil, err
	}
	manager, err := optimize.command.Manager()
	if err != nil {
		return nil, err
	}
	handler := newSMTContextCancelHandler(manager.ResourceLimit())
	context := &SMTContext{smtContextState: optimize.owner}
	scope, err := newSMTContextInterruptScope(context, handler)
	if err != nil {
		handler.close()
		return nil, err
	}
	return &apiOptimizeInterruptScope{handler: handler, scope: scope}, nil
}

func (s *apiOptimizeInterruptScope) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	result = errors.Join(result, s.scope.Close())
	s.handler.close()
	if result == nil {
		s.closed = true
		s.handler, s.scope = nil, nil
	}
	return result
}
