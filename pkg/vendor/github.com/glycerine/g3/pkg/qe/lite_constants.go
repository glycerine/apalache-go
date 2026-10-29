package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// EliminateConstants implements the source app_ref_vector overload. It uses
// equality DER alone, preserving declaration slots while unabstracting the
// constants and removing eliminated entries from the caller-owned vector.
func (l *Lite) EliminateConstants(variables *[]ast.ASTRef, formula ast.ASTRef) (err error) {
	c, err := l.begin()
	if err != nil {
		return err
	}
	defer l.finish(c, &err)
	if variables == nil {
		return ast.ErrASTHandle
	}
	if len(*variables) == 0 {
		return nil
	}
	bindings, err := l.expressions(*variables)
	if err != nil {
		return err
	}
	var sorts []ast.ASTHandle
	var names []usymbol.Symbol
	for _, h := range bindings {
		kind, err := l.manager.GetKind(h)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			return ast.ErrASTDomain
		}
		sort, err := l.manager.GetSort(h)
		if err != nil {
			return err
		}
		sorts = append(sorts, sort)
		decl, err := l.manager.GetDecl(h)
		if err != nil {
			return err
		}
		name, err := l.manager.GetASTName(decl)
		if err != nil {
			return err
		}
		names = append(names, name)
	}
	root, err := l.manager.RefGet(formula)
	if err != nil {
		return err
	}
	tmp, err := c.ref(root)
	if err != nil {
		return err
	}
	abstractor, err := ast.NewExprAbstractor(l.manager)
	if abstractor != nil {
		c.abstractors = append(c.abstractors, abstractor)
	}
	if err != nil {
		return err
	}
	if err = abstractor.Abstract(0, bindings, root, tmp); err != nil {
		return err
	}
	body, err := l.manager.RefGet(tmp)
	if err != nil {
		return err
	}
	qid, err := l.manager.Symbols.Intern("QE")
	if err != nil {
		return err
	}
	q, err := c.hold(l.manager.MkQuantifier(ast.ASTQuantifierSpec{Sorts: sorts, Names: names, Body: body, Weight: 1, QID: qid}))
	if err != nil {
		return err
	}
	proof, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = l.der.ReduceQuantifier(q, tmp, proof); err != nil {
		return err
	}
	result, err := l.manager.RefGet(tmp)
	if err != nil {
		return err
	}
	kind, err := l.manager.GetKind(result)
	if err != nil {
		return err
	}
	if kind == ast.ASTQuantifier {
		forall, err := l.manager.IsForall(result)
		if err != nil {
			return err
		}
		id, _, err := l.manager.GetQuantifierIDs(result)
		if err != nil {
			return err
		}
		if !forall && id == qid {
			body, err = l.manager.GetQuantifierBody(result)
			if err != nil {
				return err
			}
			used, err := ast.NewASTUsedVars(l.manager)
			if err != nil {
				return err
			}
			if err = used.Process(body); err != nil {
				return err
			}
			subst, err := rewriter.NewVarSubstitution(l.manager, true)
			if subst != nil {
				c.substitutions = append(c.substitutions, subst)
			}
			if err != nil {
				return err
			}
			if err = subst.Substitute(body, bindings, formula); err != nil {
				return err
			}
			j := 0
			for i, h := range bindings {
				if !used.Contains(uint32(len(bindings) - i - 1)).IsNull() {
					if err = l.manager.SetRef((*variables)[j], h); err != nil {
						return err
					}
					j++
				}
			}
			return truncateLiteVector(l.manager, variables, j)
		}
	}
	return l.manager.SetRef(formula, result)
}
