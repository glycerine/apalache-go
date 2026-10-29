package model

// Source: src/model/model_implicant.cpp:96-310.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func closeImplicantResults(m *ast.ASTManager, values *[]ast.ASTRef) error {
	for n := range *values {
		if (*values)[n] != (ast.ASTRef{}) {
			if err := m.DelRef(&(*values)[n]); err != nil {
				*values = (*values)[n:]
				return err
			}
		}
	}
	*values = nil
	return nil
}

func (i *ModelImplicant) appendResult(values *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := i.manager.NewRef(h)
	if err != nil {
		return err
	}
	*values = append(*values, r)
	return nil
}

func (i *ModelImplicant) processFormula(h ast.ASTHandle, todo, collect *[]ast.ASTHandle) error {
	if !i.isTrue(h) && !i.isFalse(h) {
		return ast.ErrASTDomain
	}
	args, err := i.appArgs(h)
	if err != nil {
		return err
	}
	family, err := i.manager.GetFamilyID(h)
	if err != nil {
		return err
	}
	if family != i.manager.BasicFamilyID() {
		*collect = append(*collect, h)
		return nil
	}
	op, err := i.manager.GetDeclKind(h)
	if err != nil {
		return err
	}
	switch op {
	case ast.ASTOpTrue, ast.ASTOpFalse:
		return nil
	case ast.ASTOpEq, ast.ASTOpIff:
		if len(args) != 2 {
			return ast.ErrASTDomain
		}
		if args[0] == args[1] {
			if !i.isTrue(h) {
				return ast.ErrASTDomain
			}
			return nil
		}
		boolean, err := i.isBool(args[0])
		if err != nil {
			return err
		}
		if boolean {
			*todo = append(*todo, args...)
		} else {
			*collect = append(*collect, h)
		}
	case ast.ASTOpDistinct:
		*collect = append(*collect, h)
	case ast.ASTOpIte:
		if len(args) != 3 {
			return ast.ErrASTDomain
		}
		if args[1] == args[2] {
			*collect = append(*collect, args[1])
		} else if i.isTrue(args[1]) && i.isTrue(args[2]) || i.isFalse(args[1]) && i.isFalse(args[2]) {
			*todo = append(*todo, args[1], args[2])
		} else if i.isTrue(args[0]) {
			*todo = append(*todo, args[0], args[1])
		} else if i.isFalse(args[0]) {
			*todo = append(*todo, args[0], args[2])
		} else {
			return ast.ErrASTDomain
		}
	case ast.ASTOpAnd:
		if i.isTrue(h) {
			*todo = append(*todo, args...)
		} else {
			for _, arg := range args {
				if i.isFalse(arg) {
					*todo = append(*todo, arg)
					return nil
				}
			}
			return ast.ErrASTDomain
		}
	case ast.ASTOpOr:
		if i.isTrue(h) {
			for _, arg := range args {
				if i.isTrue(arg) {
					*todo = append(*todo, arg)
					return nil
				}
			}
			return ast.ErrASTDomain
		}
		*todo = append(*todo, args...)
	case ast.ASTOpXor, ast.ASTOpNot:
		*todo = append(*todo, args...)
	case ast.ASTOpImplies:
		if len(args) != 2 {
			return ast.ErrASTDomain
		}
		if i.isTrue(h) {
			if i.isTrue(args[1]) {
				*todo = append(*todo, args[1])
			} else if i.isFalse(args[0]) {
				*todo = append(*todo, args[0])
			} else {
				return ast.ErrASTDomain
			}
		} else {
			*todo = append(*todo, args...)
		}
	default:
		return ast.ErrASTDomain
	}
	return nil
}

func (i *ModelImplicant) collect(formulas []ast.ASTHandle) ([]ast.ASTHandle, error) {
	for key := range i.visited {
		delete(i.visited, key)
	}
	ok, err := i.checkModel(formulas)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ast.ErrASTDomain
	}
	todo := append([]ast.ASTHandle(nil), formulas...)
	result := make([]ast.ASTHandle, 0)
	for len(todo) != 0 {
		last := len(todo) - 1
		h := todo[last]
		todo = todo[:last]
		if i.visited[h] {
			continue
		}
		kind, err := i.manager.GetKind(h)
		if err != nil {
			return nil, err
		}
		if kind != ast.ASTApp {
			return nil, ast.ErrASTDomain
		}
		if err = i.processFormula(h, &todo, &result); err != nil {
			return nil, err
		}
		i.visited[h] = true
	}
	for key := range i.visited {
		delete(i.visited, key)
	}
	return result, nil
}

// MinimizeLiterals returns owning references to the source-selected literals.
// CloseImplicantResults releases a successful result vector.
func (i *ModelImplicant) MinimizeLiterals(formulas []ast.ASTHandle, md *Model) (result []ast.ASTRef, err error) {
	if err = i.setupModel(md); err != nil {
		return nil, errors.Join(err, i.reset())
	}
	defer func() {
		err = errors.Join(err, i.reset())
		if err != nil {
			err = errors.Join(err, closeImplicantResults(i.manager, &result))
		}
	}()
	selected, err := i.collect(formulas)
	if err != nil {
		return nil, err
	}
	arithmetic, err := i.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	for _, h := range selected {
		literal := h
		if i.isFalse(h) {
			args, x := i.appArgs(h)
			if x != nil {
				return nil, x
			}
			family, x := i.manager.GetFamilyID(h)
			if x != nil {
				return nil, x
			}
			op, x := i.manager.GetDeclKind(h)
			if x != nil {
				return nil, x
			}
			if family == i.manager.BasicFamilyID() && op == ast.ASTOpEq && len(args) == 2 && i.numbers[args[0]] != nil && i.numbers[args[1]] != nil {
				left, right := args[0], args[1]
				if i.numbers[left].Cmp(i.numbers[right]) >= 0 {
					left, right = right, left
				}
				literal, x = i.manager.MkFamilyApp(arithmetic, 4, nil, []ast.ASTHandle{left, right})
				if x != nil {
					return nil, x
				}
			} else {
				literal, x = i.manager.MkNot(h)
				if x != nil {
					return nil, x
				}
			}
		}
		if err = i.appendResult(&result, literal); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (i *ModelImplicant) markCone(root ast.ASTHandle) error {
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		h := stack[last]
		stack = stack[:last]
		if i.visited[h] {
			continue
		}
		i.visited[h] = true
		kind, err := i.manager.GetKind(h)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			args, err := i.appArgs(h)
			if err != nil {
				return err
			}
			stack = append(stack, args...)
		case ast.ASTQuantifier:
			body, err := i.manager.GetQuantifierBody(h)
			if err != nil {
				return err
			}
			stack = append(stack, body)
		case ast.ASTVar:
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}

// MinimizeModel returns equalities for precisely the registered constants in
// the selected formulas' cone of influence, preserving model order.
func (i *ModelImplicant) MinimizeModel(formulas []ast.ASTHandle, md *Model) (result []ast.ASTRef, err error) {
	if err = i.setupModel(md); err != nil {
		return nil, errors.Join(err, i.reset())
	}
	defer func() {
		err = errors.Join(err, i.reset())
		if err != nil {
			err = errors.Join(err, closeImplicantResults(i.manager, &result))
		}
	}()
	selected, err := i.collect(formulas)
	if err != nil {
		return nil, err
	}
	i.states = make(map[ast.ASTHandle]implicantValue)
	for _, h := range selected {
		if err = i.markCone(h); err != nil {
			return nil, err
		}
	}
	count, err := md.Core().NumConstants()
	if err != nil {
		return nil, err
	}
	if err = i.evaluator.SetModelCompletion(true); err != nil {
		return nil, err
	}
	for n := uint32(0); n < count; n++ {
		decl, x := md.Core().Constant(n)
		if x != nil {
			return nil, x
		}
		constant, x := i.manager.MkApp(decl, nil)
		if x != nil {
			return nil, x
		}
		if !i.visited[constant] {
			continue
		}
		value, x := i.evaluator.EvaluateValue(constant)
		if x != nil {
			return nil, x
		}
		valueHandle, x := i.manager.RefGet(value)
		if x != nil {
			_ = i.manager.DelRef(&value)
			return nil, x
		}
		eq, x := i.manager.MkEq(constant, valueHandle)
		x = errors.Join(x, i.manager.DelRef(&value))
		if x != nil {
			return nil, x
		}
		if err = i.appendResult(&result, eq); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// CloseImplicantResults releases a successful MinimizeModel or
// MinimizeLiterals result vector.
func CloseImplicantResults(m *ast.ASTManager, values *[]ast.ASTRef) error {
	if m == nil || values == nil {
		return ast.ErrASTHandle
	}
	return closeImplicantResults(m, values)
}
