package fp

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"math/big"
)

// compileKarrGuardCondition builds native filter rows for one conjunct.
func (a *karrDatalogActivation) compileKarrGuardCondition(guard ast.ASTHandle, positions map[ast.ASTHandle]uint32, arity uint32) (karrGuardRule, error) {
	var out karrGuardRule
	out.condition = guard
	out.positions = cloneKarrPositions(positions)
	family, err := a.manager.GetFamilyID(guard)
	if err != nil {
		return out, err
	}
	op, err := a.manager.GetDeclKind(guard)
	if err != nil {
		return out, err
	}
	guardArity, err := a.manager.GetNumArgs(guard)
	if err != nil {
		return out, err
	}
	negated := family == a.manager.BasicFamilyID() && op == ast.ASTOpNot
	if negated {
		if guardArity != 1 {
			return out, ErrKarrDatalogFragment
		}
		guard, err = a.manager.GetArg(guard, 0)
		if err != nil {
			return out, err
		}
		family, err = a.manager.GetFamilyID(guard)
		if err != nil {
			return out, err
		}
		op, err = a.manager.GetDeclKind(guard)
		if err != nil {
			return out, err
		}
		guardArity, err = a.manager.GetNumArgs(guard)
		if err != nil {
			return out, err
		}
	}
	if guardArity != 2 {
		return out, ErrKarrDatalogFragment
	}
	if !negated && family == a.manager.BasicFamilyID() && op == ast.ASTOpOr {
		if err = a.compileKarrDisjunction(guard, positions, arity, &out); err != nil {
			return out, err
		}
		return out, nil
	}
	leftMul, rightMul := int64(1), int64(-1)
	if !negated && family == a.manager.BasicFamilyID() && op == ast.ASTOpEq {
		out.equal = true
	} else {
		arith, err := a.manager.RegisterArithmeticPlugin()
		if err != nil {
			return out, err
		}
		if family != arith || op < 2 || op > 5 {
			return out, ErrKarrDatalogFragment
		}
		if negated {
			// not (<=, >=, <, >) becomes (>, <, >=, <=).
			switch op {
			case 2:
				op = 5
			case 3:
				op = 4
			case 4:
				op = 3
			case 5:
				op = 2
			}
		}
		if op == 2 || op == 4 {
			leftMul, rightMul = -1, 1
		}
		if op == 4 || op == 5 {
			out.constant.SetInt64(-1)
		}
	}
	out.row = make([]big.Rat, arity)
	arithmetic, err := a.manager.RegisterArithmeticPlugin()
	if err != nil {
		return out, err
	}
	integer, err := a.manager.MkFamilySort(arithmetic, 1, nil)
	if err != nil {
		return out, err
	}
	add := func(expr ast.ASTHandle, factor int64) error {
		return a.addKarrLinear(expr, big.NewRat(factor, 1), positions, out.row, &out.constant, integer, arithmetic)
	}
	left, err := a.manager.GetArg(guard, 0)
	if err != nil {
		return out, err
	}
	right, err := a.manager.GetArg(guard, 1)
	if err != nil {
		return out, err
	}
	if err = add(left, leftMul); err != nil {
		return out, err
	}
	if err = add(right, rightMul); err != nil {
		return out, err
	}
	return out, nil
}

func cloneKarrPositions(source map[ast.ASTHandle]uint32) map[ast.ASTHandle]uint32 {
	result := make(map[ast.ASTHandle]uint32, len(source))
	for variable, column := range source {
		result[variable] = column
	}
	return result
}

func (a *karrDatalogActivation) relationFilterCondition(condition ast.ASTHandle, positions map[ast.ASTHandle]uint32) (result ast.ASTRef, resultErr error) {
	if condition.IsNull() {
		return result, ErrKarrDatalogFragment
	}
	if len(positions) == 0 {
		return result, fmt.Errorf("%w: filter has no relation variables", ErrKarrDatalogFragment)
	}
	family, err := a.manager.RegisterArithmeticPlugin()
	if err != nil {
		return result, err
	}
	integer, err := a.manager.MkFamilySort(family, 1, nil)
	if err != nil {
		return result, err
	}
	vars := make(map[uint32]uint32, len(positions))
	for variable, column := range positions {
		index, err := a.manager.GetVarIndex(variable)
		if err != nil {
			return result, err
		}
		vars[index] = column
	}
	var maxIndex uint32
	seenVar := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{condition}
	for len(stack) > 0 {
		h := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seenVar[h] {
			continue
		}
		seenVar[h] = true
		kind, err := a.manager.GetKind(h)
		if err != nil {
			return result, err
		}
		if kind == ast.ASTVar {
			index, err := a.manager.GetVarIndex(h)
			if err != nil {
				return result, err
			}
			_, ok := vars[index]
			if !ok {
				return result, fmt.Errorf("%w: filter variable %d has no relation column", ErrKarrDatalogFragment, index)
			}
			if index > maxIndex {
				maxIndex = index
			}
			continue
		}
		if kind != ast.ASTApp {
			continue
		}
		count, err := a.manager.GetNumArgs(h)
		if err != nil {
			return result, err
		}
		for i := uint32(0); i < count; i++ {
			child, err := a.manager.GetArg(h, i)
			if err != nil {
				return result, err
			}
			stack = append(stack, child)
		}
	}
	bindings := make([]ast.ASTHandle, maxIndex+1)
	var refs []ast.ASTRef
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, a.manager.DelRef(&refs[i]))
		}
		if resultErr != nil && result != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, a.manager.DelRef(&result))
			result = ast.ASTRef{}
		}
	}()
	for i := range bindings {
		column, ok := vars[uint32(i)]
		if !ok {
			return result, fmt.Errorf("%w: filter variable %d has no substitution", ErrKarrDatalogFragment, i)
		}
		binding, err := a.manager.MkVar(column, integer)
		if err != nil {
			return result, err
		}
		ref, err := a.manager.NewRef(binding)
		if err != nil {
			return result, err
		}
		refs = append(refs, ref)
		bindings[i] = binding
	}
	substitution, err := rewriter.NewVarSubstitution(a.manager, false)
	if substitution != nil {
		defer func() { resultErr = errors.Join(resultErr, substitution.Close()) }()
	}
	if err != nil {
		return result, err
	}
	result, err = a.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	resultErr = substitution.Substitute(condition, bindings, result)
	return result, resultErr
}
