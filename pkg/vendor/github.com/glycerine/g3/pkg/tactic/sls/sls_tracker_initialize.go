package sls

// Source: src/tactic/sls/sls_tracker.h:300-397,430-479.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// initializeApp builds the source score-table and uplink entries for one app.
// The graph walker invokes it once per identity, after visiting the children.
func (t *Tracker) initializeApp(node ast.ASTHandle) (result error) {
	if _, err := t.trackExpr(node); err != nil {
		return err
	}
	n, err := t.manager.GetNumArgs(node)
	if err != nil {
		return err
	}
	for i := uint32(0); i < n; i++ {
		child, e := t.manager.GetArg(node, i)
		if e != nil {
			return e
		}
		t.uplinks[child] = append(t.uplinks[child], node)
	}
	if n != 0 {
		return nil
	}
	family, err := t.manager.GetFamilyID(node)
	if err != nil {
		return err
	}
	if family != ast.NullFamilyID {
		value := numeral.NewInt(0)
		defer func() { result = errors.Join(result, t.numbers.Del(&value)) }()
		if err = t.valueToInteger(node, &value); err != nil {
			return err
		}
		return t.SetValue(node, &value)
	}
	decl, err := t.manager.GetDecl(node)
	if err != nil {
		return err
	}
	if _, found := t.entryPoints[decl]; !found {
		t.entryPoints[decl] = node
	}
	if _, found := t.constantSet[decl]; !found {
		t.constantSet[decl] = struct{}{}
		t.constants = append(t.constants, decl)
	}
	return nil
}

// valueToInteger ports the retained Boolean and BV numeral branches of
// value2mpz. Other interpreted constants remain explicit dependency work.
func (t *Tracker) valueToInteger(node ast.ASTHandle, out *numeral.Int) (result error) {
	family, err := t.manager.GetFamilyID(node)
	if err != nil {
		return err
	}
	op, err := t.manager.GetDeclKind(node)
	if err != nil {
		return err
	}
	if family == t.manager.BasicFamilyID() {
		switch op {
		case ast.ASTOpTrue:
			return t.numbers.Copy(out, &t.one)
		case ast.ASTOpFalse:
			return t.numbers.Copy(out, &t.zero)
		default:
			return ErrEvaluatorUnsupported
		}
	}
	bvFamily, err := t.manager.BitVectorUtilityFamilyID()
	if err != nil {
		return err
	}
	if family != bvFamily || op != slsBVNum {
		return ErrEvaluatorUnsupported
	}
	service := t.manager.NumeralService()
	if service == nil {
		return ast.ErrASTHandle
	}
	value, err := service.NewValue()
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, service.CleanupValue(&value)) }()
	var width uint32
	matched, err := t.manager.BitVectorNumeralValue(node, &value, &width)
	if err != nil {
		return err
	}
	if !matched {
		return ErrEvaluatorUnsupported
	}
	one, err := service.Manager.Z.IsOne(&value.Raw.Den)
	if err != nil {
		return err
	}
	if !one {
		return ErrEvaluatorUnsupported
	}
	return t.numbers.CopyFrom(out, service.Manager.Z, &value.Raw.Num)
}

// InitializeExpressions ports the retained for_each_expr initialization,
// uplink construction, top-expression publication and distance pass.
func (t *Tracker) InitializeExpressions(roots []ast.ASTHandle) error {
	if err := t.open(); err != nil {
		return err
	}
	visited := make(map[ast.ASTHandle]struct{})
	var visit func(ast.ASTHandle) error
	visit = func(node ast.ASTHandle) error {
		if _, found := visited[node]; found {
			return nil
		}
		visited[node] = struct{}{}
		kind, err := t.manager.GetKind(node)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			// Source init_proc deliberately ignores variables and quantifiers.
			return nil
		}
		n, err := t.manager.GetNumArgs(node)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			child, e := t.manager.GetArg(node, i)
			if e != nil {
				return e
			}
			if e = visit(child); e != nil {
				return e
			}
		}
		return t.initializeApp(node)
	}
	for _, root := range roots {
		kind, err := t.manager.GetKind(root)
		if err != nil {
			return err
		}
		if kind != ast.ASTApp {
			return ast.ErrASTDomain
		}
		if err = visit(root); err != nil {
			return err
		}
		t.topExpressions[root] = struct{}{}
		if _, found := t.weights[root]; !found {
			t.weights[root] = t.pawsInit
		}
	}
	if err := t.calculateExprDistances(roots); err != nil {
		return err
	}
	if t.ucbInit {
		t.touched = uint32(len(roots))
	} else {
		t.touched = 1
	}
	return nil
}

func (t *Tracker) calculateExprDistances(roots []ast.ASTHandle) error {
	stack := append([]ast.ASTHandle(nil), roots...)
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		score, err := t.existingScore(current)
		if err != nil {
			return err
		}
		n, err := t.manager.GetNumArgs(current)
		if err != nil {
			return err
		}
		for i := uint32(0); i < n; i++ {
			child, e := t.manager.GetArg(current, i)
			if e != nil {
				return e
			}
			childScore, e := t.existingScore(child)
			if e != nil {
				return e
			}
			if score.distance >= childScore.distance {
				childScore.distance = score.distance + 1
				stack = append(stack, child)
			}
		}
	}
	return nil
}

func (t *Tracker) Constants() ([]ast.ASTHandle, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), t.constants...), nil
}
