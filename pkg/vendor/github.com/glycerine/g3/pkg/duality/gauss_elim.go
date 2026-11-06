package duality

// Source: duality_rpfp.cpp, VariableProjector::GaussElimCheap.
import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

type projectionCandidate struct {
	variable, value ast.ASTHandle
	dependencies    int
}

// GaussElimCheap eliminates acyclic constant-equality definitions in source
// dependency order. Keep terms and input literals are borrowed; output terms
// are independently owned. Cyclic definitions are left in the formula.
func (g *RPFP) GaussElimCheap(keep, literals []*Expr, simplifier *interp.Manager) (out []*Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if simplifier == nil {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	owner, err := simplifier.ASTManager()
	if err != nil {
		return nil, err
	}
	if owner != m {
		return nil, ErrContextMismatch
	}
	var refs []ast.ASTRef
	var outputs []*Expr
	defer func() {
		for i := range refs {
			result = errors.Join(result, m.DelRef(&refs[i]))
		}
		if result != nil {
			for _, term := range outputs {
				result = errors.Join(result, term.Close())
			}
			out = nil
		}
	}()
	raw := func(term *Expr) (ast.ASTHandle, error) {
		if err := term.open(); err != nil {
			return ast.ASTHandle{}, err
		}
		if term.value.object.context != g.context {
			return ast.ASTHandle{}, ErrContextMismatch
		}
		return term.Raw()
	}
	kept := make(map[ast.ASTHandle]bool)
	order := make(map[ast.ASTHandle]int)
	for _, term := range keep {
		h, err := raw(term)
		if err != nil {
			return nil, err
		}
		kept[h] = true
		order[h] = len(order)
	}
	varNum := func(h ast.ASTHandle) int {
		if n, ok := order[h]; ok {
			return n
		}
		n := len(order)
		order[h] = n
		return n
	}
	isVar := func(h ast.ASTHandle) (bool, error) {
		kind, err := m.GetKind(h)
		if err != nil || kind != ast.ASTApp {
			return false, err
		}
		family, err := m.GetFamilyID(h)
		if err != nil {
			return false, err
		}
		n, err := m.GetNumArgs(h)
		return family == ast.NullFamilyID && n == 0, err
	}
	inputs := make([]ast.ASTHandle, len(literals))
	for i, term := range literals {
		inputs[i], err = raw(term)
		if err != nil {
			return nil, err
		}
	}
	var candidates []projectionCandidate
	indices := make(map[ast.ASTHandle]int)
	newCandidate := func(lhs, rhs ast.ASTHandle) {
		indices[lhs] = len(candidates)
		candidates = append(candidates, projectionCandidate{variable: lhs, value: rhs})
	}
	var makeCandidate func(ast.ASTHandle, ast.ASTHandle) error
	makeCandidate = func(lhs, rhs ast.ASTHandle) error {
		if lhs == rhs {
			return nil
		}
		lv, err := isVar(lhs)
		if err != nil {
			return err
		}
		rv, err := isVar(rhs)
		if err != nil {
			return err
		}
		if !lv {
			if rv {
				return makeCandidate(rhs, lhs)
			}
			return nil
		}
		if rv {
			rn := varNum(rhs)
			ln := varNum(lhs)
			if rn > ln {
				return makeCandidate(rhs, lhs)
			}
		}
		if kept[lhs] {
			return nil
		}
		if index, found := indices[lhs]; found {
			if rv {
				if _, found := indices[rhs]; !found && !kept[rhs] {
					newCandidate(rhs, candidates[index].value)
				}
			}
			candidates[index].value = rhs
		} else {
			newCandidate(lhs, rhs)
		}
		return nil
	}
	var findRep func(ast.ASTHandle) (ast.ASTHandle, error)
	findRep = func(h ast.ASTHandle) (ast.ASTHandle, error) {
		index, found := indices[h]
		if !found {
			return h, nil
		}
		value := candidates[index].value
		variable, err := isVar(value)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if variable {
			if varNum(value) >= varNum(h) {
				return ast.ASTHandle{}, ErrWrapperOwner
			}
			rep, err := findRep(value)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			candidates[index].value = rep
			return rep, nil
		}
		return h, nil
	}
	for _, literal := range inputs {
		kind, err := m.GetKind(literal)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := m.GetFamilyID(literal)
		if err != nil {
			return nil, err
		}
		op, err := m.GetDeclKind(literal)
		if err != nil {
			return nil, err
		}
		if family != m.BasicFamilyID() || (op != ast.ASTOpEq && op != ast.ASTOpIff) {
			continue
		}
		lhs, err := m.GetArg(literal, 0)
		if err != nil {
			return nil, err
		}
		rhs, err := m.GetArg(literal, 1)
		if err != nil {
			return nil, err
		}
		lhs, err = findRep(lhs)
		if err != nil {
			return nil, err
		}
		rhs, err = findRep(rhs)
		if err != nil {
			return nil, err
		}
		if err := makeCandidate(lhs, rhs); err != nil {
			return nil, err
		}
	}
	supporters := make(map[ast.ASTHandle][]int)
	var ready []int
	for i := range candidates {
		seen := make(map[ast.ASTHandle]bool)
		var visit func(ast.ASTHandle) error
		visit = func(h ast.ASTHandle) error {
			if seen[h] {
				return nil
			}
			seen[h] = true
			kind, err := m.GetKind(h)
			if err != nil {
				return err
			}
			if kind == ast.ASTQuantifier {
				body, err := m.GetQuantifierBody(h)
				if err != nil {
					return err
				}
				return visit(body)
			}
			if kind != ast.ASTApp {
				return nil
			}
			variable, err := isVar(h)
			if err != nil {
				return err
			}
			if variable {
				if _, found := indices[h]; found {
					candidates[i].dependencies++
					supporters[h] = append(supporters[h], i)
				}
			}
			n, err := m.GetNumArgs(h)
			if err != nil {
				return err
			}
			for j := uint32(0); j < n; j++ {
				a, err := m.GetArg(h, j)
				if err != nil {
					return err
				}
				if err := visit(a); err != nil {
					return err
				}
			}
			return nil
		}
		if err := visit(candidates[i].value); err != nil {
			return nil, err
		}
		if candidates[i].dependencies == 0 {
			ready = append(ready, i)
		}
	}
	eliminated := make(map[ast.ASTHandle]ast.ASTHandle)
	substitute := func(h ast.ASTHandle) (ast.ASTHandle, error) {
		ref, err := substituteExpression(g.context, h, eliminated)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		refs = append(refs, ref)
		return m.RefGet(ref)
	}
	for len(ready) > 0 {
		i := ready[len(ready)-1]
		ready = ready[:len(ready)-1]
		rep, err := findRep(candidates[i].variable)
		if err != nil {
			return nil, err
		}
		if rep != candidates[i].variable {
			if j, found := indices[rep]; found {
				candidates[i].value = candidates[j].value
			}
		}
		value, err := substitute(candidates[i].value)
		if err != nil {
			return nil, err
		}
		eliminated[candidates[i].variable] = value
		for _, j := range supporters[candidates[i].variable] {
			candidates[j].dependencies--
			if candidates[j].dependencies == 0 {
				ready = append(ready, j)
			}
		}
	}
	var propLiteral func(ast.ASTHandle) (bool, error)
	propLiteral = func(h ast.ASTHandle) (bool, error) {
		variable, err := isVar(h)
		if err != nil || variable {
			return variable, err
		}
		negative, child, err := m.IsNot(h)
		if err != nil {
			return false, err
		}
		if negative {
			return propLiteral(child)
		}
		return false, nil
	}
	for _, literal := range inputs {
		h, err := substitute(literal)
		if err != nil {
			return nil, err
		}
		ref, err := simplifier.Simplify(h)
		if err != nil {
			return nil, err
		}
		refs = append(refs, ref)
		h, err = m.RefGet(ref)
		if err != nil {
			return nil, err
		}
		if m.IsTrue(h) {
			continue
		}
		prop, err := propLiteral(h)
		if err != nil {
			return nil, err
		}
		if prop && !kept[h] {
			continue
		}
		term, err := NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, term)
	}
	return outputs, nil
}
