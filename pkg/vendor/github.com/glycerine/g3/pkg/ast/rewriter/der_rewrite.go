package rewriter

// Source: src/ast/rewriter/der.cpp:100-206,317-461;
// src/ast/rewriter/der.h:120-181.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *DER) isOr(h ast.ASTHandle) (bool, error) {
	return d.isBasic(h, ast.ASTOpOr)
}

func (d *DER) substituteTerm(c *derCall, term ast.ASTHandle, bindings []ast.ASTHandle, scratch ast.ASTRef) (ast.ASTHandle, error) {
	if err := d.substitute.Substitute(term, bindings, scratch); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := d.manager.RefGet(scratch)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.hold(value)
}

func (d *DER) applySubstitution(c *derCall, q, body ast.ASTHandle, positions []int32, definitions []ast.ASTHandle, order []uint32) (result ast.ASTHandle, err error) {
	bindings := make([]ast.ASTHandle, len(definitions))
	scratch, err := d.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() { err = errors.Join(err, d.manager.DelRef(&scratch)) }()
	for _, index := range order {
		current, x := d.substituteTerm(c, definitions[index], bindings, scratch)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		bindings[len(bindings)-int(index)-1] = current
	}
	count, err := d.manager.GetNumArgs(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	args := make([]ast.ASTHandle, 0, count)
	for i := uint32(0); i < count; i++ {
		index := positions[i]
		if index >= 0 && definitions[index] != (ast.ASTHandle{}) {
			continue
		}
		arg, x := d.manager.GetArg(body, i)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		args = append(args, arg)
	}
	var reducedBody ast.ASTHandle
	if len(args) == 1 {
		reducedBody = args[0]
	} else {
		reducedBody, err = d.manager.MkOrArray(args)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	reducedBody, err = d.substituteTerm(c, reducedBody, bindings, scratch)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patternsN, err := d.manager.GetNumPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	patterns := make([]ast.ASTHandle, patternsN)
	for i := range patterns {
		old, x := d.manager.GetPattern(q, uint32(i))
		if x != nil {
			return ast.ASTHandle{}, x
		}
		patterns[i], x = d.substituteTerm(c, old, bindings, scratch)
		if x != nil {
			return ast.ASTHandle{}, x
		}
	}
	noPatternsN, err := d.manager.GetNumNoPatterns(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	noPatterns := make([]ast.ASTHandle, noPatternsN)
	for i := range noPatterns {
		old, x := d.manager.GetNoPattern(q, uint32(i))
		if x != nil {
			return ast.ASTHandle{}, x
		}
		noPatterns[i], x = d.substituteTerm(c, old, bindings, scratch)
		if x != nil {
			return ast.ASTHandle{}, x
		}
	}
	return d.manager.UpdateQuantifierAnnotations(q, patterns, noPatterns, reducedBody)
}

func (d *DER) reduceOnce(c *derCall, q ast.ASTHandle) (ast.ASTHandle, error) {
	forall, err := d.manager.IsForall(q)
	if err != nil || !forall {
		return q, err
	}
	body, err := d.manager.GetQuantifierBody(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declarations, err := d.manager.GetNumDeclarations(q)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	isOr, err := d.isOr(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !isOr {
		variable, _, definition, matched, err := d.isVarDiseq(c, body, declarations)
		if err != nil || !matched {
			return q, err
		}
		occurs, err := d.manager.OccursExpr(variable, definition)
		if err != nil || occurs {
			return q, err
		}
		return d.manager.MkFalse()
	}

	count, err := d.manager.GetNumArgs(body)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	positions := make([]int32, count)
	for i := range positions {
		positions[i] = -1
	}
	definitions := make([]ast.ASTHandle, declarations)
	variables := make([]ast.ASTHandle, declarations)
	disequalities := uint32(0)
	for i := uint32(0); i < count; i++ {
		arg, x := d.manager.GetArg(body, i)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		variable, index, definition, matched, x := d.isVarDiseq(c, arg, declarations)
		if x != nil {
			return ast.ASTHandle{}, x
		}
		if matched && definitions[index] == (ast.ASTHandle{}) {
			definitions[index] = definition
			variables[index] = variable
			positions[i] = int32(index)
			disequalities++
		}
	}
	if disequalities == 0 {
		return q, nil
	}
	order, err := d.sortVariables(variables, definitions)
	if err != nil || len(order) == 0 {
		return q, err
	}
	return d.applySubstitution(c, q, body, positions, definitions, order)
}

// ReduceQuantifier applies DER to a universal quantifier to a fixed point and
// then removes declarations made unused by the simultaneous substitution.
func (d *DER) ReduceQuantifier(q ast.ASTHandle, out, proof ast.ASTRef) (result error) {
	if err := d.open(); err != nil {
		return err
	}
	if kind, err := d.manager.GetKind(q); err != nil {
		return err
	} else if kind != ast.ASTQuantifier {
		return ast.ErrASTDomain
	}
	call := derCall{owner: d}
	defer func() { result = errors.Join(result, call.close()) }()
	current := q
	reduced := false
	for {
		next, err := d.reduceOnce(&call, current)
		if err != nil {
			return err
		}
		if next == current {
			break
		}
		reduced = true
		current, err = call.hold(next)
		if err != nil {
			return err
		}
		kind, err := d.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind != ast.ASTQuantifier {
			break
		}
	}
	if reduced {
		kind, err := d.manager.GetKind(current)
		if err != nil {
			return err
		}
		forall := false
		if kind == ast.ASTQuantifier {
			forall, err = d.manager.IsForall(current)
			if err != nil {
				return err
			}
		}
		if kind == ast.ASTQuantifier && forall {
			eliminated, err := ElimUnusedVarsUFBV(d.manager, current)
			if err != nil {
				return err
			}
			defer func() { result = errors.Join(result, d.manager.DelRef(&eliminated)) }()
			current, err = d.manager.RefGet(eliminated)
			if err != nil {
				return err
			}
		}
	}
	if err := d.manager.SetRef(out, current); err != nil {
		return err
	}
	proofs, err := d.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	if !reduced {
		return d.manager.ResetRef(proof)
	}
	p, err := d.manager.MkProofDER(q, current)
	if err != nil {
		return err
	}
	return d.manager.SetRef(proof, p)
}

type DERConfiguration struct {
	DefaultConfiguration
	self   *DERConfiguration
	core   *DER
	closed bool
}

func NewDERConfiguration(m *ast.ASTManager) (*DERConfiguration, error) {
	d, err := NewDER(m)
	if err != nil {
		return nil, err
	}
	c := &DERConfiguration{core: d}
	c.self = c
	c.defaults.manager = m
	return c, nil
}

func (c *DERConfiguration) ReduceQuantifier(q, body ast.ASTHandle, patterns, noPatterns ast.AppArguments, out, proof ast.ASTRef) (result bool, err error) {
	if c == nil || c.self != c || c.closed || c.core == nil {
		return false, ast.ErrASTHandle
	}
	updated, err := c.core.manager.UpdateQuantifierArguments(q, body, patterns, noPatterns)
	if err != nil {
		return false, err
	}
	ref, err := c.core.manager.NewRef(updated)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, c.core.manager.DelRef(&ref)) }()
	return true, c.core.ReduceQuantifier(updated, out, proof)
}

func (c *DERConfiguration) Close() error {
	if c == nil || c.self != c {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	c.closed = true
	return c.core.Close()
}

type DERRewriter struct {
	self      *DERRewriter
	manager   *ast.ASTManager
	config    *DERConfiguration
	traversal *Traversal
	closed    bool
}

func NewDERRewriter(m *ast.ASTManager) (*DERRewriter, error) {
	c, err := NewDERConfiguration(m)
	if err != nil {
		return nil, err
	}
	r := &DERRewriter{manager: m, config: c}
	r.self = r
	r.traversal, err = NewTraversal(m, c)
	return r, err
}

func (r *DERRewriter) Rewrite(root ast.ASTHandle, out, proof ast.ASTRef) error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	if err := r.traversal.Rewrite(root, out); err != nil {
		return err
	}
	proofs, err := r.manager.ProofsEnabled()
	if err != nil || !proofs {
		return err
	}
	value, err := r.manager.RefGet(out)
	if err != nil {
		return err
	}
	var p ast.ASTHandle
	if value == root {
		p, err = r.manager.MkProofReflexivity(root)
	} else {
		p, err = r.manager.MkProofRewrite(root, value)
	}
	if err != nil {
		return err
	}
	return r.manager.SetRef(proof, p)
}

func (r *DERRewriter) Reset() error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	return r.traversal.Reset()
}

func (r *DERRewriter) Cleanup() error {
	if r == nil || r.self != r || r.closed || r.traversal == nil {
		return ast.ErrASTHandle
	}
	if err := r.traversal.Cleanup(); err != nil {
		return err
	}
	if r.config == nil || r.config.core == nil || r.config.core.substitute == nil {
		return ast.ErrASTHandle
	}
	return r.config.core.substitute.reducer.engine.Cleanup()
}

func (r *DERRewriter) Close() (result error) {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.traversal != nil {
		result = errors.Join(result, r.traversal.Close())
	}
	if r.config != nil {
		result = errors.Join(result, r.config.Close())
	}
	r.closed = true
	return result
}
