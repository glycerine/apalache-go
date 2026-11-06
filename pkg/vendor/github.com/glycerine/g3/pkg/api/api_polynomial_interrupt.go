package api

// Source: cancel_eh, context::set_interruptable and scoped_timer in
// src/api/api_polynomial.cpp:61-68.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
)

type apiOperationInterruptScope struct {
	handler *smtContextCancelHandler
	scope   *smtContextInterruptScope
	timer   *control.ScopedTimer
	closed  bool
}

func newAPIOperationInterruptScope(context *SMTContext) (*apiOperationInterruptScope, error) {
	manager, err := context.Manager()
	if err != nil {
		return nil, err
	}
	return newAPIInterruptScopeForLimit(context, manager.ResourceLimit())
}

func newAPIInterruptScopeForLimit(context *SMTContext, limit *control.Reslimit) (*apiOperationInterruptScope, error) {
	timeout, err := context.contextTimeout()
	if err != nil {
		return nil, err
	}
	handler := newSMTContextCancelHandler(limit)
	scope, err := newSMTContextInterruptScope(context, handler)
	if err != nil {
		handler.close()
		return nil, err
	}
	timer, err := control.NewScopedTimer(timeout, handler)
	if err != nil {
		return nil, errors.Join(err, scope.Close())
	}
	return &apiOperationInterruptScope{handler: handler, scope: scope, timer: timer}, nil
}

func (s *apiOperationInterruptScope) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	result = errors.Join(result, s.timer.Close(), s.scope.Close())
	s.handler.close()
	if result == nil {
		s.closed = true
		s.handler, s.scope, s.timer = nil, nil, nil
	}
	return result
}

type apiPolynomialInterruptScope = apiOperationInterruptScope

func newAPIPolynomialInterruptScope(context *SMTContext) (*apiPolynomialInterruptScope, error) {
	limit, err := context.polynomialLimit()
	if err != nil {
		return nil, err
	}
	return newAPIInterruptScopeForLimit(context, limit)
}
