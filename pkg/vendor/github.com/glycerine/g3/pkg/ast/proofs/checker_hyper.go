package proofs

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

func (c *Checker) setFalse(e ast.ASTHandle, position uint32) (ast.ASTHandle, ast.ASTHandle, error) {
	if args, yes, err := c.op(e, ast.ASTOpOr, -1); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	} else if yes {
		if position >= uint32(len(args)) {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ErrASTDomain
		}
		args = append([]ast.ASTHandle(nil), args...)
		lit := args[position]
		f, err := c.manager.MkFalse()
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		args[position] = f
		r, err := c.manager.MkOrArray(args)
		return r, lit, err
	}
	if args, yes, err := c.op(e, ast.ASTOpImplies, 2); err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	} else if yes {
		body, head := args[0], args[1]
		heads, isOr, err := c.op(head, ast.ASTOpOr, -1)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if !isOr {
			heads = []ast.ASTHandle{head}
		}
		bodies, isAnd, err := c.op(body, ast.ASTOpAnd, -1)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		if !isAnd {
			bodies = []ast.ASTHandle{body}
		}
		if position < uint32(len(heads)) {
			heads = append([]ast.ASTHandle(nil), heads...)
			lit := heads[position]
			f, err := c.manager.MkFalse()
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			heads[position] = f
			h, err := c.manager.MkOrArray(heads)
			if err != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, err
			}
			r, err := c.manager.MkImplies(body, h)
			return r, lit, err
		}
		position -= uint32(len(heads))
		if position >= uint32(len(bodies)) {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ErrASTDomain
		}
		bodies = append([]ast.ASTHandle(nil), bodies...)
		not, err := c.manager.MkNot(bodies[position])
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		truth, err := c.manager.MkTrue()
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		bodies[position] = truth
		b, err := c.manager.MkAndArray(bodies)
		if err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, err
		}
		r, err := c.manager.MkImplies(b, head)
		return r, not, err
	}
	if position != 0 {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ErrASTDomain
	}
	f, err := c.manager.MkFalse()
	return f, e, err
}

func (c *Checker) checkHyperResolve(p ast.ASTHandle) (valid bool, side []ast.ASTHandle, result error) {
	conclusion, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, nil, err
	}
	defer func() { result = joinResult(result, c.manager.DelRef(&conclusion)) }()
	out, err := ast.NewASTProofHyperOutput(c.manager)
	if err != nil {
		return false, nil, err
	}
	defer func() { result = joinResult(result, out.Close()) }()
	var premises []ast.ASTHandle
	defer func() {
		for _, q := range premises {
			result = joinResult(result, c.manager.DecRef(q))
		}
	}()
	yes, err := c.manager.MatchProofHyperResolve(p, &premises, conclusion, out)
	if err != nil || !yes {
		return false, nil, err
	}
	if len(premises) == 0 || out.PositionsLen()+1 != uint32(len(premises)) || out.SubstitutionsLen() != uint32(len(premises)) {
		return false, nil, nil
	}
	vs, err := rewriter.NewVarSubstitution(c.manager, false)
	if err != nil {
		return false, nil, err
	}
	defer func() { result = joinResult(result, vs.Close()) }()
	tmp, err := c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, nil, err
	}
	defer func() { result = joinResult(result, c.manager.DelRef(&tmp)) }()
	var formulas, pinned []ast.ASTHandle
	defer func() {
		for _, f := range pinned {
			result = joinResult(result, c.manager.DecRef(f))
		}
	}()
	for i, q := range premises {
		f, has, e := c.fact(q)
		if e != nil || !has {
			return false, nil, e
		}
		sub, e := out.Substitution(uint32(i))
		if e != nil {
			return false, nil, e
		}
		if len(sub) != 0 {
			if kind, e := c.manager.GetKind(f); e != nil {
				return false, nil, e
			} else if kind == ast.ASTQuantifier {
				f, e = c.manager.GetQuantifierBody(f)
				if e != nil {
					return false, nil, e
				}
			}
			if e := vs.Substitute(f, sub, tmp); e != nil {
				return false, nil, e
			}
			f, e = c.manager.RefGet(tmp)
			if e != nil {
				return false, nil, e
			}
		}
		if e := c.manager.IncRef(f); e != nil {
			return false, nil, e
		}
		formulas = append(formulas, f)
		pinned = append(pinned, f)
	}
	premise0 := formulas[0]
	for i := 1; i < len(formulas); i++ {
		pos, e := out.Position(uint32(i - 1))
		if e != nil {
			return false, nil, e
		}
		var lit1, lit2 ast.ASTHandle
		premise0, lit1, e = c.setFalse(premise0, pos.First)
		if e != nil {
			return false, nil, e
		}
		formulas[i], lit2, e = c.setFalse(formulas[i], pos.Second)
		if e != nil {
			return false, nil, e
		}
		// Source only diagnoses non-complementary literals; the generated side
		// condition remains the validation obligation.
		_, _ = c.negated(lit1, lit2)
	}
	formulas[0] = premise0
	resolved, err := c.manager.MkOrArray(formulas)
	if err != nil {
		return false, nil, err
	}
	conclusionHandle, err := c.manager.RefGet(conclusion)
	if err != nil {
		return false, nil, err
	}
	if kind, e := c.manager.GetKind(conclusionHandle); e != nil {
		return false, nil, e
	} else if kind == ast.ASTQuantifier {
		body, e := c.manager.GetQuantifierBody(conclusionHandle)
		if e != nil {
			return false, nil, e
		}
		iff, e := c.manager.MkIff(resolved, body)
		if e != nil {
			return false, nil, e
		}
		n, e := c.manager.GetNumDeclarations(conclusionHandle)
		if e != nil {
			return false, nil, e
		}
		sorts := make([]ast.ASTHandle, 0, n)
		names := make([]usymbol.Symbol, 0, n)
		for i := uint32(0); i < n; i++ {
			s, e := c.manager.GetDeclarationSort(conclusionHandle, i)
			if e != nil {
				return false, nil, e
			}
			sorts = append(sorts, s)
			name, e := c.manager.GetDeclarationName(conclusionHandle, i)
			if e != nil {
				return false, nil, e
			}
			names = append(names, name)
		}
		resolved, e = c.manager.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: iff})
		if e != nil {
			return false, nil, e
		}
	} else {
		resolved, err = c.manager.MkIff(resolved, conclusionHandle)
		if err != nil {
			return false, nil, err
		}
	}
	return true, []ast.ASTHandle{resolved}, nil
}

func joinResult(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
