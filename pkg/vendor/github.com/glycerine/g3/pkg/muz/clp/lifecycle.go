package clp

import "github.com/glycerine/g3/pkg/ast"

// Statistics returns the source counters, which its search leaves unchanged.
func (e *Engine) Statistics() (Statistics, error) {
	if err := e.open(); err != nil {
		return Statistics{}, err
	}
	return e.stats, nil
}
func (e *Engine) ResetStatistics() error {
	if err := e.open(); err != nil {
		return err
	}
	e.stats = Statistics{}
	return nil
}
func (e *Engine) Answer() (ast.ASTHandle, error) {
	if err := e.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return e.manager.MkTrue()
}
func (e *Engine) Close() error {
	if e == nil {
		return ErrCLP
	}
	if e.closed {
		return nil
	}
	e.closed = true
	e.manager = nil
	e.rules = nil
	e.solver = nil
	return nil
}
