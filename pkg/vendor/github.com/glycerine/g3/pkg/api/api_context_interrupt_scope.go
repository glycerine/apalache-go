package api

// Source: api::context::set_interruptable in src/api/api_context.cpp:121-137.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
)

var errSMTContextInterruptScope = errors.New("api: interrupt handler is already installed")

type smtContextInterruptScope struct {
	owner   *smtContextState
	handler control.EventHandler
	closed  bool
}

func newSMTContextInterruptScope(context *SMTContext, handler control.EventHandler) (*smtContextInterruptScope, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, errSMTContextInterruptScope
	}
	if function, ok := handler.(control.EventHandlerFunc); ok && function == nil {
		return nil, errSMTContextInterruptScope
	}
	owner := context.smtContextState
	owner.interruptMu.Lock()
	defer owner.interruptMu.Unlock()
	if owner.interruptable != nil {
		return nil, errSMTContextInterruptScope
	}
	scope := &smtContextInterruptScope{owner: owner, handler: handler}
	owner.interruptable = handler
	owner.interruptScope = scope
	return scope, nil
}

func (s *smtContextInterruptScope) Close() error {
	if s == nil || s.closed {
		return nil
	}
	if s.owner == nil {
		return ErrSMTContextOwner
	}
	s.owner.interruptMu.Lock()
	defer s.owner.interruptMu.Unlock()
	if s.owner.interruptScope != s {
		return errSMTContextInterruptScope
	}
	s.owner.interruptable = nil
	s.owner.interruptScope = nil
	s.closed = true
	s.owner, s.handler = nil, nil
	return nil
}
