package smtcore

// Source: src/smt/smt_internalizer.cpp:28-181.

import "github.com/glycerine/g3/pkg/ast"

type SMTInternalizationItem struct {
	Expression  ast.ASTHandle
	GateContext bool
}

type SMTInternalizationOrder struct {
	manager             *ast.ASTManager
	context             *SMTCheckerContext
	booleanInternalized map[ast.ASTHandle]bool
	nonDefaultFamilies  map[ast.FamilyID]bool
}

func NewSMTInternalizationOrder(manager *ast.ASTManager, context *SMTCheckerContext) (*SMTInternalizationOrder, error) {
	if manager == nil || context == nil || context.manager != manager {
		return nil, ast.ErrASTHandle
	}
	return &SMTInternalizationOrder{manager: manager, context: context, booleanInternalized: make(map[ast.ASTHandle]bool), nonDefaultFamilies: make(map[ast.FamilyID]bool)}, nil
}

func (o *SMTInternalizationOrder) SetBooleanInternalized(expression ast.ASTHandle, internalized bool) error {
	if o == nil || o.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := o.manager.GetKind(expression); err != nil {
		return err
	}
	o.booleanInternalized[expression] = internalized
	return nil
}

func (o *SMTInternalizationOrder) SetNonDefaultFamily(family ast.FamilyID, nonDefault bool) error {
	if o == nil || o.manager == nil || family == ast.NullFamilyID || family == o.manager.BasicFamilyID() {
		return ast.ErrASTDomain
	}
	o.nonDefaultFamilies[family] = nonDefault
	return nil
}

func (o *SMTInternalizationOrder) isBoolean(expression ast.ASTHandle) (bool, error) {
	sort, err := o.manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	boolean, err := o.manager.MkBoolSort()
	return sort == boolean, err
}

func (o *SMTInternalizationOrder) isGate(expression ast.ASTHandle) (bool, error) {
	kind, err := o.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := o.manager.GetFamilyID(expression)
	if err != nil || family != o.manager.BasicFamilyID() {
		return false, err
	}
	op, err := o.manager.GetDeclKind(expression)
	if err != nil {
		return false, err
	}
	return op == ast.ASTOpAnd || op == ast.ASTOpOr || op == ast.ASTOpIff || op == ast.ASTOpIte, nil
}

func (o *SMTInternalizationOrder) visitChildren(item SMTInternalizationItem, colors map[SMTInternalizationItem]uint8, todo *[]SMTInternalizationItem) (bool, error) {
	kind, err := o.manager.GetKind(item.Expression)
	if err != nil {
		return false, err
	}
	if kind == ast.ASTQuantifier {
		return true, nil
	}
	if kind != ast.ASTApp {
		return false, ast.ErrASTDomain
	}
	boolean, err := o.isBoolean(item.Expression)
	if err != nil {
		return false, err
	}
	if boolean && o.booleanInternalized[item.Expression] || !boolean && o.context.enodes[item.Expression] != nil {
		return true, nil
	}
	family, err := o.manager.GetFamilyID(item.Expression)
	if err != nil {
		return false, err
	}
	// Source theory internalizers consume their own-family descendants top-down.
	if family != ast.NullFamilyID && o.nonDefaultFamilies[family] {
		return true, nil
	}
	count, err := o.manager.GetNumArgs(item.Expression)
	if err != nil {
		return false, err
	}
	operation, err := o.manager.GetDeclKind(item.Expression)
	if err != nil {
		return false, err
	}
	termITE := operation == ast.ASTOpIte && !boolean
	visited := true
	push := func(expression ast.ASTHandle, gate bool) {
		child := SMTInternalizationItem{Expression: expression, GateContext: gate}
		if colors[child] == 0 {
			*todo = append(*todo, child)
			visited = false
		}
	}
	if termITE {
		for index, gate := range []bool{true, false, false} {
			child, e := o.manager.GetArg(item.Expression, uint32(index))
			if e != nil {
				return false, e
			}
			push(child, gate)
		}
		return visited, nil
	}
	gate, err := o.isGate(item.Expression)
	if err != nil {
		return false, err
	}
	newGate := boolean && gate
	if family == o.manager.BasicFamilyID() && operation == ast.ASTOpNot {
		newGate = boolean
	}
	for index := count; index > 0; index-- {
		child, e := o.manager.GetArg(item.Expression, index-1)
		if e != nil {
			return false, e
		}
		push(child, newGate)
	}
	return visited, nil
}

func (o *SMTInternalizationOrder) Sort(root ast.ASTHandle) ([]SMTInternalizationItem, error) {
	if o == nil || o.manager == nil {
		return nil, ast.ErrASTHandle
	}
	rootItem := SMTInternalizationItem{Expression: root, GateContext: true}
	todo := []SMTInternalizationItem{rootItem}
	colors := make(map[SMTInternalizationItem]uint8)
	var sorted []SMTInternalizationItem
	for len(todo) != 0 {
		item := todo[len(todo)-1]
		switch colors[item] {
		case 0:
			colors[item] = 1
			if _, err := o.visitChildren(item, colors, &todo); err != nil {
				return nil, err
			}
		case 1:
			visited, err := o.visitChildren(item, colors, &todo)
			if err != nil {
				return nil, err
			}
			if !visited {
				continue
			}
			colors[item] = 2
			op, opErr := o.manager.GetDeclKind(item.Expression)
			if item.Expression != root && (opErr != nil || op != ast.ASTOpNot) {
				sorted = append(sorted, item)
			}
		case 2:
			todo = todo[:len(todo)-1]
		default:
			return nil, ast.ErrASTDomain
		}
	}
	return sorted, nil
}
