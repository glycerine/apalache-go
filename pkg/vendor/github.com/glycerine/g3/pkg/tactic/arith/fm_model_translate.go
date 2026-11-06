package arith

// Source: src/tactic/model_converter.h translation contract applied to fm converter state.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (c *FMModelConverter) Translate(tr *ast.ASTTranslation) (_ tactic.ModelConverter, err error) {
	if e := c.open(); e != nil {
		return nil, e
	}
	if tr == nil || tr.From() != c.manager || tr.To() == nil {
		return nil, ast.ErrASTHandle
	}
	services, e := arithmeticTargetServices(tr.To(), c.services)
	if e != nil {
		return nil, e
	}
	r, e := NewFMModelConverter(tr.To(), services)
	if e != nil {
		return nil, e
	}
	done := false
	defer func() {
		if !done {
			err = errors.Join(err, r.Close())
		}
	}()
	for i, v := range c.variables {
		tv, e := tr.Translate(v)
		if e != nil {
			return nil, e
		}
		row := make([]ast.ASTHandle, len(c.clauses[i]))
		for j, h := range c.clauses[i] {
			row[j], e = tr.Translate(h)
			if e != nil {
				return nil, e
			}
		}
		if e = r.Insert(tv, row); e != nil {
			return nil, e
		}
	}
	done = true
	return r, nil
}
