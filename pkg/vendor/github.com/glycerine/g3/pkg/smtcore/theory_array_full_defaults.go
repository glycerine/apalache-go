package smtcore

// Source: src/smt/theory_array_full.cpp instantiate_default_const_axiom,
// instantiate_default_map_axiom, instantiate_default_store_axiom,
// instantiate_default_as_array_axiom, and add_parent_default.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// The epsilon value's declaration also owns its result sort, keeping the
// borrowed sort key valid until the cache entry is removed.
func (t *SMTArrayTheory) restoreArrayEpsilons(limit int) error {
	if limit < 0 || limit > len(t.epsilonTrail) {
		return ast.ErrASTDomain
	}
	for len(t.epsilonTrail) > limit {
		sort := t.epsilonTrail[len(t.epsilonTrail)-1]
		if err := t.state.manager.DecRef(t.epsilons[sort]); err != nil {
			return err
		}
		delete(t.epsilons, sort)
		t.epsilonTrail = t.epsilonTrail[:len(t.epsilonTrail)-1]
	}
	return nil
}

func (t *SMTArrayTheory) instantiateConstructorDefault(constructor ast.ASTHandle) (added bool, err error) {
	s, err := t.State()
	if err != nil {
		return false, err
	}
	op, array, err := t.arrayOperation(constructor)
	if err != nil || !array {
		return false, err
	}
	if op != ast.ASTArrayOpConst && op != ast.ASTArrayOpMap && op != ast.ASTArrayOpStore && op != ast.ASTArrayOpAsArray {
		return false, nil
	}
	node := s.context.enodes[constructor]
	if node == nil {
		return false, ast.ErrASTDomain
	}
	// All default constructors share the source's (this, node) fingerprint
	// shape. Check both the original node and its current root snapshot.
	if t.defaultFingerprints[node] || t.defaultFingerprints[node.root] {
		return false, nil
	}
	t.defaultFingerprints[node.root] = true
	t.defaultFingerprintTrail = append(t.defaultFingerprintTrail, node.root)
	line, function := 0, ""
	switch op {
	case ast.ASTArrayOpConst:
		t.statistics.defaultConst++
		line, function = 573, "instantiate_default_const_axiom"
	case ast.ASTArrayOpMap:
		t.statistics.defaultMap++
		line, function = 546, "instantiate_default_map_axiom"
	case ast.ASTArrayOpStore:
		t.statistics.defaultStore++
		line, function = 707, "instantiate_default_store_axiom"
	case ast.ASTArrayOpAsArray:
		t.statistics.defaultAsArray++
		line, function = 587, "instantiate_default_as_array_axiom"
	}
	if err = t.traceFullArrayExpression(function, line, constructor); err != nil {
		return false, err
	}
	value, err := t.defaultConstructorValue(constructor, op)
	if err != nil {
		return false, err
	}
	ref, err := s.manager.NewRef(value)
	if err != nil {
		return false, err
	}
	defer func() { err = errors.Join(err, s.manager.DelRef(&ref)) }()
	if op == ast.ASTArrayOpMap && s.theoryRewriter != nil {
		if err = s.theoryRewriter.Rewrite(value, ref); err != nil {
			return false, err
		}
		if value, err = s.manager.RefGet(ref); err != nil {
			return false, err
		}
	}
	defaultTerm, err := s.manager.MkFamilyApp(t.FamilyID(), ast.ASTArrayOpDefault, nil, []ast.ASTHandle{constructor})
	if err != nil {
		return false, err
	}
	if err = s.Internalize(defaultTerm, false); err != nil {
		return false, err
	}
	if err = s.Internalize(value, false); err != nil {
		return false, err
	}
	if op == ast.ASTArrayOpConst || op == ast.ASTArrayOpAsArray {
		return t.tryAssignArrayEquality(value, defaultTerm)
	}
	return t.tryAssignArrayEquality(defaultTerm, value)
}
