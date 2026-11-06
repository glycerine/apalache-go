package bv

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
)

func (r *bvArrayRewriter) reduce(h ast.ASTHandle, args []ast.ASTHandle) (ast.ASTHandle, bool, error) {
	m := r.owner.manager
	d, e := m.GetDecl(h)
	if e != nil {
		return h, false, e
	}
	family, e := m.GetFamilyID(d)
	if e != nil {
		return h, false, e
	}
	op, e := m.GetDeclKind(d)
	if e != nil {
		return h, false, e
	}
	rangeSort, e := m.GetRange(d)
	if e != nil {
		return h, false, e
	}
	rangeArray, e := r.isBVArraySort(rangeSort)
	if e != nil {
		return h, false, e
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpEq && len(args) == 2 {
		array, e := r.isBVArray(args[0])
		if e != nil || !array {
			return h, false, e
		}
		if args[0] == args[1] {
			truth, e := m.MkTrue()
			return truth, true, e
		}
		q, e := r.extensionalEquality(args[0], args[1])
		return q, true, e
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpDistinct && len(args) > 0 {
		array, e := r.isBVArray(args[0])
		if e != nil || !array {
			return h, false, e
		}
		expanded, e := m.MkDistinctExpanded(args)
		if e != nil {
			return h, false, e
		}
		expanded, e = r.rewrite(expanded)
		return expanded, true, e
	}
	if family == m.BasicFamilyID() && op == ast.ASTOpIte && rangeArray && len(args) == 3 {
		result, e := r.addDefinition(h, func(x ast.ASTHandle) (ast.ASTHandle, error) {
			ft, e := r.functionFor(args[1])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			ff, e := r.functionFor(args[2])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			tv, e := r.apply(ft, x)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			fv, e := r.apply(ff, x)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return m.MkIte(args[0], tv, fv)
		})
		return result, true, e
	}
	if family == m.BasicFamilyID() && rangeArray {
		return h, false, tactic.NewTacticException("bvarray2uf: basic array-valued application is not implemented")
	}
	if family == ast.NullFamilyID {
		hasArray := false
		for _, a := range args {
			yes, x := r.isBVArray(a)
			if x != nil {
				return h, false, x
			}
			hasArray = hasArray || yes
		}
		if rangeArray {
			f, x := r.functionFor(h)
			if x != nil {
				return h, false, x
			}
			v, x := m.MkAsArray(f)
			return v, true, x
		}
		if hasArray {
			return h, true, nil
		}
		return h, false, nil
	}
	if family != r.owner.arrayFamily {
		return h, false, nil
	}
	switch op {
	case ast.ASTArrayOpSelect:
		if len(args) < 2 {
			return h, false, ast.ErrASTDomain
		}
		yes, x := r.isBVArray(args[0])
		if x != nil || !yes {
			return h, false, x
		}
		f, x := r.functionFor(args[0])
		if x != nil {
			return h, false, x
		}
		index, x := r.packedIndex(args[1:])
		if x != nil {
			return h, false, x
		}
		v, x := r.apply(f, index)
		return v, true, x
	case ast.ASTArrayOpConst:
		if len(args) != 1 || !rangeArray {
			return h, false, nil
		}
		v, x := r.addDefinition(h, func(ast.ASTHandle) (ast.ASTHandle, error) { return args[0], nil })
		return v, true, x
	case ast.ASTArrayOpStore:
		if len(args) < 3 {
			return h, false, ast.ErrASTDomain
		}
		yes, x := r.isBVArray(args[0])
		if x != nil || !yes {
			return h, false, x
		}
		stored := args[0]
		index, x := r.packedIndex(args[1 : len(args)-1])
		if x != nil {
			return h, false, x
		}
		value := args[len(args)-1]
		fs, x := r.functionFor(stored)
		if x != nil {
			return h, false, x
		}
		ft, x := r.functionFor(h)
		if x != nil {
			return h, false, x
		}
		sort, x := m.GetSort(h)
		if x != nil {
			return h, false, x
		}
		indexSort, x := r.indexSort(sort)
		if x != nil {
			return h, false, x
		}
		variable, x := m.MkVar(0, indexSort)
		if x != nil {
			return h, false, x
		}
		same, x := m.MkEq(variable, index)
		if x != nil {
			return h, false, x
		}
		newAt, x := r.apply(ft, variable)
		if x != nil {
			return h, false, x
		}
		oldAt, x := r.apply(fs, variable)
		if x != nil {
			return h, false, x
		}
		unchanged, x := m.MkEq(newAt, oldAt)
		if x != nil {
			return h, false, x
		}
		body, x := m.MkOr(same, unchanged)
		if x != nil {
			return h, false, x
		}
		quantified, x := r.forall(indexSort, body)
		if x != nil {
			return h, false, x
		}
		if x = r.addExtra(quantified); x != nil {
			return h, false, x
		}
		result, x := m.MkAsArray(ft)
		if x != nil {
			return h, false, x
		}
		at, x := r.apply(ft, index)
		if x != nil {
			return h, false, x
		}
		ground, x := m.MkEq(at, value)
		if x == nil {
			x = r.addExtra(ground)
		}
		return result, true, x
	case ast.ASTArrayOpMap:
		mapf, x := m.ArrayMapFunction(d)
		if x != nil {
			return h, false, x
		}
		funcs := make([]ast.ASTHandle, len(args))
		for i, a := range args {
			funcs[i], x = r.functionFor(a)
			if x != nil {
				return h, false, x
			}
		}
		v, x := r.addDefinition(h, func(index ast.ASTHandle) (ast.ASTHandle, error) {
			values := make([]ast.ASTHandle, len(funcs))
			for i, f := range funcs {
				values[i], x = r.apply(f, index)
				if x != nil {
					return ast.ASTHandle{}, x
				}
			}
			return m.MkApp(mapf, values)
		})
		return v, true, x
	case ast.ASTArrayOpAsArray:
		return h, false, nil
	}
	return h, false, tactic.NewTacticException("bvarray2uf: array operation is not implemented")
}
