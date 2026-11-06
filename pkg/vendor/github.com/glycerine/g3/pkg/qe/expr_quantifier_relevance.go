package qe

// Source: src/qe/qe.cpp, quant_elim_plugin::get_max_relevant.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// maxRelevantExpr owns both the rebuilt formula and its relevant subformula.
type maxRelevantExpr struct {
	manager             *ast.ASTManager
	formula, subformula ast.ASTRef
}

func extractMaxRelevantExpr(m *ast.ASTManager, adapter *SolverNNFAdapter, formula ast.ASTHandle) (result *maxRelevantExpr, err error) {
	if m == nil || adapter == nil {
		return nil, ast.ErrASTHandle
	}
	result = &maxRelevantExpr{manager: m}
	defer func() {
		if err != nil {
			err = errors.Join(err, result.Close())
			result = nil
		}
	}()
	result.formula, err = m.NewRef(formula)
	if err != nil {
		return result, err
	}
	result.subformula, err = m.NewRef(formula)
	if err != nil {
		return result, err
	}
	var visit func(ast.ASTHandle) error
	visit = func(root ast.ASTHandle) error {
		kind, e := m.GetKind(root)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp {
			if e = m.SetRef(result.formula, root); e != nil {
				return e
			}
			return m.SetRef(result.subformula, root)
		}
		family, e := m.GetFamilyID(root)
		if e != nil {
			return e
		}
		op, e := m.GetDeclKind(root)
		if e != nil {
			return e
		}
		if family != m.BasicFamilyID() || (op != ast.ASTOpAnd && op != ast.ASTOpOr) {
			if e = m.SetRef(result.formula, root); e != nil {
				return e
			}
			return m.SetRef(result.subformula, root)
		}
		count, e := m.GetNumArgs(root)
		if e != nil {
			return e
		}
		var relevant, irrelevant []ast.ASTHandle
		for i := uint32(0); i < count; i++ {
			arg, x := m.GetArg(root, i)
			if x != nil {
				return x
			}
			yes, x := adapter.IsRelevant(arg)
			if x != nil {
				return x
			}
			if yes {
				relevant = append(relevant, arg)
			} else {
				irrelevant = append(irrelevant, arg)
			}
		}
		if len(relevant) == 0 || len(irrelevant) == 0 {
			if e = m.SetRef(result.formula, root); e != nil {
				return e
			}
			return m.SetRef(result.subformula, root)
		}
		decl, e := m.GetDecl(root)
		if e != nil {
			return e
		}
		if len(relevant) == 1 {
			if e = visit(relevant[0]); e != nil {
				return e
			}
			inner, x := m.RefGet(result.formula)
			if x != nil {
				return x
			}
			irrelevant = append(irrelevant, inner)
		} else {
			sub, x := m.MkApp(decl, relevant)
			if x != nil {
				return x
			}
			if x = m.SetRef(result.subformula, sub); x != nil {
				return x
			}
			irrelevant = append(irrelevant, sub)
		}
		rebuilt, e := m.MkApp(decl, irrelevant)
		if e != nil {
			return e
		}
		return m.SetRef(result.formula, rebuilt)
	}
	if err = visit(formula); err != nil {
		return result, err
	}
	return result, nil
}
func (r *maxRelevantExpr) Close() error {
	if r == nil || r.manager == nil {
		return nil
	}
	for _, ref := range []*ast.ASTRef{&r.subformula, &r.formula} {
		if *ref != (ast.ASTRef{}) {
			if err := r.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	r.manager = nil
	return nil
}
