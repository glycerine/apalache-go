package cmd_context

// Source: src/api/api_datalog.cpp fixedpoint query timeout scopes.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/control"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type fixedpointTimeoutScope struct {
	handler *contextTimeoutHandler
	timer   *control.ScopedTimer
}

func (c *Context) fixedpointTimeout(params *uparams.ParamsRef) (value uint32, resultErr error) {
	module, err := c.config.Globals.SourceGetModuleName("fixedpoint")
	if err != nil {
		return 0, err
	}
	defer func() { resultErr = errors.Join(resultErr, c.config.Params.Del(&module)) }()
	name := "timeout"
	defaultTimeout, err := c.config.Params.GetUIntName(&c.params, &name, math.MaxUint32)
	if err != nil {
		return 0, err
	}
	if params == nil {
		return c.config.Params.GetUIntName(&module, &name, defaultTimeout)
	}
	return c.config.Params.GetUIntFallbackName(params, &name, &module, defaultTimeout)
}

func (c *Context) beginFixedpointTimeout(params *uparams.ParamsRef) (*fixedpointTimeoutScope, error) {
	timeout, err := c.fixedpointTimeout(params)
	if err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	handler := &contextTimeoutHandler{limit: manager.ResourceLimit(), active: true}
	timer, err := control.NewScopedTimer(timeout, handler)
	if err != nil {
		handler.close()
		return nil, err
	}
	return &fixedpointTimeoutScope{handler: handler, timer: timer}, nil
}

func (s *fixedpointTimeoutScope) Close() error {
	if s == nil || s.timer == nil {
		return nil
	}
	err := s.timer.Close()
	s.handler.close()
	s.timer, s.handler = nil, nil
	return err
}
