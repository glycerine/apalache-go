package rel

import (
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
)

func (b *AAGBuilder) Expression(expression ast.ASTHandle) (uint32, error) {
	if e := b.open(); e != nil {
		return 0, e
	}
	if id, ok := b.expressions[expression]; ok {
		return id, nil
	}
	kind, e := b.manager.GetKind(expression)
	if e != nil {
		return 0, e
	}
	if kind == ast.ASTVar {
		return b.Input(expression)
	}
	if kind != ast.ASTApp {
		return 0, fmt.Errorf("%w: unsupported expression kind", ErrAAG)
	}
	op, e := b.manager.GetDeclKind(expression)
	if e != nil {
		return 0, e
	}
	count, e := b.manager.GetNumArgs(expression)
	if e != nil {
		return 0, e
	}
	switch op {
	case ast.ASTOpFalse:
		return 0, nil
	case ast.ASTOpTrue:
		return 1, nil
	case ast.ASTOpNot:
		if count != 1 {
			return 0, ErrAAG
		}
		a, e := b.manager.GetArg(expression, 0)
		if e != nil {
			return 0, e
		}
		id, e := b.Expression(a)
		return AAGNegate(id), e
	case ast.ASTOpOr, ast.ASTOpAnd:
		ids := make([]uint32, count)
		for i := uint32(0); i < count; i++ {
			a, x := b.manager.GetArg(expression, i)
			if x != nil {
				return 0, x
			}
			ids[i], x = b.Expression(a)
			if x != nil {
				return 0, x
			}
		}
		var id uint32
		if op == ast.ASTOpOr {
			id, e = b.OrMany(ids)
		} else {
			id, e = b.AndMany(ids)
		}
		if e == nil {
			b.expressions[expression] = id
		}
		return id, e
	default:
		return b.Input(expression)
	}
}
