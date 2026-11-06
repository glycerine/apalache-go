package cmd_context

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/control"
)

type optimizationExecutionScope struct {
	limitScope control.LimitScope
	handler    *contextTimeoutHandler
	timer      *control.ScopedTimer
	closed     bool
}

func (c *Context) beginOptimizationExecution() (*optimizationExecutionScope, error) {
	settings, err := c.optimizationSettings()
	if err != nil {
		return nil, err
	}
	limit := c.manager.ResourceLimit()
	limitScope, err := control.NewLimitScope(limit, settings.rlimit)
	if err != nil {
		return nil, err
	}
	handler := &contextTimeoutHandler{limit: limit, active: true}
	timer, err := control.NewScopedTimer(settings.timeout, handler)
	if err != nil {
		handler.close()
		limitScope.Close()
		return nil, err
	}
	return &optimizationExecutionScope{limitScope: limitScope, handler: handler, timer: timer}, nil
}

func (s *optimizationExecutionScope) Close() (result error) {
	if s == nil || s.closed {
		return nil
	}
	s.closed = true
	if s.timer != nil {
		result = errors.Join(result, s.timer.Close())
	}
	if s.handler != nil {
		s.handler.close()
	}
	s.limitScope.Close()
	s.timer, s.handler = nil, nil
	return result
}
