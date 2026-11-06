package rel

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type AAGFact struct {
	Declaration ast.ASTHandle
	Values      []ast.ASTHandle
}
type AAGExporter struct {
	manager             *ast.ASTManager
	rules               *base.RuleSet
	facts               []AAGFact
	builder             *AAGBuilder
	predicateIDs        map[ast.ASTHandle]uint32
	current, next       []ast.ASTHandle
	argCurrent, argNext []ast.ASTHandle
	refs                []ast.ASTRef
	closed              bool
}

func NewAAGExporter(manager *ast.ASTManager, rules *base.RuleSet, facts []AAGFact) (*AAGExporter, error) {
	if manager == nil || rules == nil {
		return nil, ErrAAG
	}
	b, e := NewAAGBuilder(manager)
	if e != nil {
		return nil, e
	}
	x := &AAGExporter{manager: manager, rules: rules, facts: append([]AAGFact(nil), facts...), builder: b, predicateIDs: make(map[ast.ASTHandle]uint32)}
	if e = x.initializePredicates(); e != nil {
		return nil, errors.Join(e, x.Close())
	}
	return x, nil
}
func (x *AAGExporter) open() error {
	if x == nil || x.closed || x.builder == nil {
		return ErrAAG
	}
	return nil
}
func (x *AAGExporter) pin(h ast.ASTHandle) error {
	r, e := x.manager.NewRef(h)
	if e == nil {
		x.refs = append(x.refs, r)
	}
	return e
}
func (x *AAGExporter) fresh(prefix string) (ast.ASTHandle, error) {
	s, e := x.manager.MkBoolSort()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	h, e := x.manager.MkFreshConst(prefix, s)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return h, x.pin(h)
}
func (x *AAGExporter) Close() (result error) {
	if x == nil {
		return ErrAAG
	}
	if x.closed {
		return nil
	}
	for i := len(x.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, x.manager.DelRef(&x.refs[i]))
	}
	if x.builder != nil {
		result = errors.Join(result, x.builder.Close())
	}
	x.builder = nil
	x.refs = nil
	x.closed = true
	return result
}
