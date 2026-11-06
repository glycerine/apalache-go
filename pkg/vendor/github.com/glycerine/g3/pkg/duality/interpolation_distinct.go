package duality

// Source: src/duality/duality.h iZ3LogicSolver::assert_axiom.

import (
	"errors"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// interpolationAxiom applies the source legacy-solver workaround to a
// top-level distinct with more than ten arguments. The generated constraints
// are equivalent: each argument receives a distinct integer image.
func (g *RPFP) interpolationAxiom(manager *interp.Manager, axiom *Expr) (result *Expr, changed bool, resultErr error) {
	if err := axiom.open(); err != nil {
		return nil, false, err
	}
	h, err := axiom.Raw()
	if err != nil {
		return nil, false, err
	}
	op, err := manager.Operator(h)
	if err != nil {
		return nil, false, err
	}
	if op != interp.OperatorDistinct {
		return axiom, false, nil
	}
	args, err := manager.Args(h)
	if err != nil {
		return nil, false, err
	}
	if len(args) <= 10 {
		return axiom, false, nil
	}
	m := g.context.manager
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	pin := func(value ast.ASTHandle) error {
		ref, e := m.NewRef(value)
		if e == nil {
			refs = append(refs, ref)
		}
		return e
	}
	zero, err := manager.MakeIntText("0")
	if err != nil {
		return nil, false, err
	}
	if err = pin(zero); err != nil {
		return nil, false, err
	}
	integerSort, err := m.GetSort(zero)
	if err != nil {
		return nil, false, err
	}
	argumentSort, err := m.GetSort(args[0])
	if err != nil {
		return nil, false, err
	}
	prefix, err := m.Symbols.Intern("@distinct")
	if err != nil {
		return nil, false, err
	}
	declaration, err := m.MkFreshFuncDecl(prefix, usymbol.Symbol{}, []ast.ASTHandle{argumentSort}, integerSort)
	if err != nil {
		return nil, false, err
	}
	if err = pin(declaration); err != nil {
		return nil, false, err
	}
	constraints := make([]ast.ASTHandle, len(args))
	for i, arg := range args {
		image, e := m.MkApp(declaration, []ast.ASTHandle{arg})
		if e != nil {
			return nil, false, e
		}
		if e = pin(image); e != nil {
			return nil, false, e
		}
		index, e := manager.MakeIntText(strconv.Itoa(i))
		if e != nil {
			return nil, false, e
		}
		if e = pin(index); e != nil {
			return nil, false, e
		}
		constraints[i], e = m.MkEq(image, index)
		if e != nil {
			return nil, false, e
		}
		if e = pin(constraints[i]); e != nil {
			return nil, false, e
		}
	}
	conjunction, err := m.MkAndArray(constraints)
	if err != nil {
		return nil, false, err
	}
	result, err = NewExpr(g.context, conjunction)
	if err != nil {
		return nil, false, err
	}
	return result, true, nil
}
