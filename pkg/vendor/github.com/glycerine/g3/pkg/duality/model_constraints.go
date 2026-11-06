package duality

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

// ModelValueAsConstraint records an existing scalar interpretation or explicit
// finite-array entries. Source intentionally omits the array default value and
// does not complete missing scalar interpretations.
// Source: duality_rpfp.cpp:2623-2645.
func (g *RPFP) ModelValueAsConstraint(term *Expr) (out *Expr, result error) {
	if err := g.open(); err != nil {
		return nil, err
	}
	if err := term.open(); err != nil {
		return nil, err
	}
	if term.value.object.context != g.context {
		return nil, ErrContextMismatch
	}
	if g.dualModel == nil {
		return nil, ErrWrapperOwner
	}
	m := g.context.manager
	h, err := term.Raw()
	if err != nil {
		return nil, err
	}
	sortHandle, err := m.GetSort(h)
	if err != nil {
		return nil, err
	}
	family, err := m.GetFamilyID(sortHandle)
	if err != nil {
		return nil, err
	}
	array, err := m.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	if family == array {
		value, err := g.evaluate(term)
		if err != nil {
			return nil, err
		}
		defer func() {
			result = errors.Join(result, value.Close())
			if result != nil && out != nil {
				result = errors.Join(result, out.Close())
				out = nil
			}
		}()
		vh, err := value.Raw()
		if err != nil {
			return nil, err
		}
		finite, err := g.evalArrayTerm(vh)
		if err != nil {
			return nil, err
		}
		if finite.defined {
			return g.modelArrayEntriesAsConstraint(h, finite)
		}
	} else {
		decl, err := m.GetDecl(h)
		if err != nil {
			return nil, err
		}
		value, err := g.dualModel.ConstantInterpretation(decl)
		if err != nil {
			return nil, err
		}
		if !value.IsNull() {
			equality, err := m.MkEq(h, value)
			if err != nil {
				return nil, err
			}
			return NewExpr(g.context, equality)
		}
	}
	return g.context.BoolVal(true)
}

// EvalNodeAsConstraint renames model constraints on indexed variables back to
// the node's original parameters and returns an independently owned relation.
// Source: duality_rpfp.cpp:2647-2665.
func (g *RPFP) EvalNodeAsConstraint(node *Node) (out *Transformer, result error) {
	activation, variables, err := g.RedVars(node)
	if err != nil {
		return nil, err
	}
	var pins []*Expr
	pins = append(pins, activation)
	pins = append(pins, variables...)
	defer func() {
		for _, pin := range pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	m := g.context.manager
	var constraints []ast.ASTHandle
	memo := make(map[ast.ASTHandle]ast.ASTHandle)
	var parameters []*Expr
	for i, variable := range variables {
		constraint, err := g.ModelValueAsConstraint(variable)
		if err != nil {
			return nil, err
		}
		pins = append(pins, constraint)
		h, err := constraint.Raw()
		if err != nil {
			return nil, err
		}
		if !m.IsTrue(h) {
			constraints = append(constraints, h)
		}
		original, err := m.RefGet(node.Annotation.individualParams[i])
		if err != nil {
			return nil, err
		}
		parameter, err := NewExpr(g.context, original)
		if err != nil {
			return nil, err
		}
		pins = append(pins, parameter)
		parameters = append(parameters, parameter)
		indexed, err := variable.Raw()
		if err != nil {
			return nil, err
		}
		memo[indexed] = original
	}
	var combined ast.ASTHandle
	switch len(constraints) {
	case 0:
		combined, err = m.MkTrue()
	case 1:
		combined = constraints[0]
	default:
		combined, err = m.MkAndArgs(constraints)
	}
	if err != nil {
		return nil, err
	}
	ref, err := substituteExpression(g.context, combined, memo)
	if err != nil {
		return nil, err
	}
	defer func() { result = errors.Join(result, m.DelRef(&ref)) }()
	h, err := m.RefGet(ref)
	if err != nil {
		return nil, err
	}
	formula, err := NewExpr(g.context, h)
	if err != nil {
		return nil, err
	}
	pins = append(pins, formula)
	return NewTransformer(g.context, nil, parameters, formula)
}

// modelArrayEntriesAsConstraint consumes borrowed model entries while their owner is live.
func (g *RPFP) modelArrayEntriesAsConstraint(h ast.ASTHandle, finite arrayModelValue) (out *Expr, result error) {
	m := g.context.manager
	array, err := m.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	var pins []*Expr
	defer func() {
		for _, pin := range pins {
			result = errors.Join(result, pin.Close())
		}
		if result != nil && out != nil {
			result = errors.Join(result, out.Close())
			out = nil
		}
	}()
	wrap := func(h ast.ASTHandle, err error) (*Expr, error) {
		if err != nil {
			return nil, err
		}
		term, err := NewExpr(g.context, h)
		if err != nil {
			return nil, err
		}
		pins = append(pins, term)
		return term, nil
	}
	keys := make([]ast.ASTHandle, 0, len(finite.entries))
	for key := range finite.entries {
		keys = append(keys, key)
	}
	// Source std::map orders opaque AST pointers; use stable arena identity.
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Slot() != keys[j].Slot() {
			return keys[i].Slot() < keys[j].Slot()
		}
		return keys[i].Generation() < keys[j].Generation()
	})
	var constraints []ast.ASTHandle
	for _, key := range keys {
		selected, err := wrap(m.MkFamilyApp(array, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{h, key}))
		if err != nil {
			return nil, err
		}
		sh, err := selected.Raw()
		if err != nil {
			return nil, err
		}
		equality, err := wrap(m.MkEq(sh, finite.entries[key]))
		if err != nil {
			return nil, err
		}
		eh, err := equality.Raw()
		if err != nil {
			return nil, err
		}
		constraints = append(constraints, eh)
	}
	var combined ast.ASTHandle
	switch len(constraints) {
	case 0:
		combined, err = m.MkTrue()
	case 1:
		combined = constraints[0]
	default:
		combined, err = m.MkAndArgs(constraints)
	}
	if err != nil {
		return nil, err
	}
	return NewExpr(g.context, combined)
}
