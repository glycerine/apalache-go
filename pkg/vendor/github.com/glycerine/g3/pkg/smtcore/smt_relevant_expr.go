package smtcore

// Source: src/smt/smt_for_each_relevant_expr.h and
// src/smt/smt_for_each_relevant_expr.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/symbol"
)

func smtAtLabelCount(m *ast.ASTManager, expression ast.ASTHandle, polarity bool) (uint32, error) {
	var names []symbol.Symbol
	matched, err := m.IsLabelLit(expression, &names)
	if err != nil {
		return 0, err
	}
	if !matched {
		var positive bool
		matched, err = m.IsLabel(expression, &positive, &names)
		if err != nil {
			return 0, err
		}
		matched = matched && positive == polarity
	}
	if !matched {
		return 0, nil
	}
	var count uint32
	for _, name := range names {
		contains, err := m.Symbols.Contains(name, '@')
		if err != nil {
			return 0, err
		}
		if contains {
			count++
		}
	}
	return count, nil
}

func smtAtLabelOperator(m *ast.ASTManager, expression ast.ASTHandle) (int32, bool, error) {
	kind, err := m.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return 0, false, err
	}
	family, err := m.GetFamilyID(expression)
	if err != nil || family != m.BasicFamilyID() {
		return 0, false, err
	}
	op, err := m.GetDeclKind(expression)
	return op, err == nil, err
}

func smtCountAtLabels(m *ast.ASTManager, expression ast.ASTHandle, positive bool) (uint32, error) {
	count, err := smtAtLabelCount(m, expression, positive)
	if err != nil {
		return 0, err
	}
	op, basic, err := smtAtLabelOperator(m, expression)
	if err != nil || !basic {
		return count, err
	}
	n, err := m.GetNumArgs(expression)
	if err != nil {
		return 0, err
	}
	arg := func(index uint32, polarity bool) (uint32, error) {
		child, err := m.GetArg(expression, index)
		if err != nil {
			return 0, err
		}
		return smtCountAtLabels(m, child, polarity)
	}
	combineSum := op == ast.ASTOpAnd && positive || op == ast.ASTOpOr && !positive
	combineMax := op == ast.ASTOpOr && positive || op == ast.ASTOpAnd && !positive
	switch {
	case combineSum:
		for i := uint32(0); i < n; i++ {
			childCount, err := arg(i, positive)
			if err != nil {
				return 0, err
			}
			count += childCount
		}
	case combineMax:
		for i := uint32(0); i < n; i++ {
			childCount, err := arg(i, positive)
			if err != nil {
				return 0, err
			}
			if childCount > count {
				count = childCount
			}
		}
	case op == ast.ASTOpNot:
		if n != 1 {
			return 0, ast.ErrASTDomain
		}
		return arg(0, !positive)
	case op == ast.ASTOpImplies:
		if n != 2 {
			return 0, ast.ErrASTDomain
		}
		left, err := arg(0, !positive)
		if err != nil {
			return 0, err
		}
		right, err := arg(1, positive)
		if err != nil {
			return 0, err
		}
		if positive {
			if left > count {
				count = left
			}
			if right > count {
				count = right
			}
		} else {
			count += left + right
		}
	}
	return count, nil
}

// SMTCheckAtLabels reports whether every and-or path contains at most one
// positive @ label, matching check_at_labels::check.
func SMTCheckAtLabels(m *ast.ASTManager, constraint ast.ASTHandle) (bool, error) {
	if m == nil {
		return false, ast.ErrASTHandle
	}
	count, err := smtCountAtLabels(m, constraint, true)
	return count <= 1, err
}

// SMTRelevantExprContext is the context state consumed by the relevant-term
// traversal. The full SMT context owns and updates this concrete component.
type SMTRelevantExprContext struct {
	manager      *ast.ASTManager
	relevant     map[ast.ASTHandle]bool
	internalized map[ast.ASTHandle]bool
	assignments  map[ast.ASTHandle]lbool.LBool
	cache        map[ast.ASTHandle]struct{}
}

func NewSMTRelevantExprContext(m *ast.ASTManager) (*SMTRelevantExprContext, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTRelevantExprContext{
		manager: m, relevant: make(map[ast.ASTHandle]bool),
		internalized: make(map[ast.ASTHandle]bool), assignments: make(map[ast.ASTHandle]lbool.LBool),
		cache: make(map[ast.ASTHandle]struct{}),
	}, nil
}

func (c *SMTRelevantExprContext) SetRelevant(expression ast.ASTHandle, relevant bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := c.manager.GetKind(expression); err != nil {
		return err
	}
	c.relevant[expression] = relevant
	return nil
}

func (c *SMTRelevantExprContext) SetAssignment(expression ast.ASTHandle, value lbool.LBool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if value != lbool.False && value != lbool.Undef && value != lbool.True {
		return ast.ErrASTDomain
	}
	if _, err := c.manager.GetKind(expression); err != nil {
		return err
	}
	c.internalized[expression] = true
	c.assignments[expression] = value
	return nil
}

func (c *SMTRelevantExprContext) SetNotInternalized(expression ast.ASTHandle) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := c.manager.GetKind(expression); err != nil {
		return err
	}
	delete(c.internalized, expression)
	delete(c.assignments, expression)
	return nil
}

func (c *SMTRelevantExprContext) Reset() {
	if c != nil {
		c.cache = make(map[ast.ASTHandle]struct{})
	}
}

func (c *SMTRelevantExprContext) assignment(expression ast.ASTHandle) (lbool.LBool, error) {
	if !c.internalized[expression] {
		return lbool.True, nil
	}
	value, found := c.assignments[expression]
	if !found {
		return lbool.Undef, ast.ErrASTDomain
	}
	return value, nil
}

func (c *SMTRelevantExprContext) arguments(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := c.manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	arguments := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		arguments[i], err = c.manager.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
	}
	return arguments, nil
}

func (c *SMTRelevantExprContext) pushAll(expression ast.ASTHandle, todo *[]ast.ASTHandle) error {
	arguments, err := c.arguments(expression)
	if err != nil {
		return err
	}
	for _, argument := range arguments {
		if !c.relevant[argument] {
			return ast.ErrASTDomain
		}
		if _, found := c.cache[argument]; !found {
			*todo = append(*todo, argument)
		}
	}
	return nil
}

func (c *SMTRelevantExprContext) pushRelevantChild(expression ast.ASTHandle, value lbool.LBool, todo *[]ast.ASTHandle) error {
	arguments, err := c.arguments(expression)
	if err != nil {
		return err
	}
	for _, argument := range arguments {
		if !c.relevant[argument] {
			continue
		}
		assignment, err := c.assignment(argument)
		if err != nil {
			return err
		}
		if assignment == value {
			if _, found := c.cache[argument]; found {
				return nil
			}
		}
	}
	for _, argument := range arguments {
		if !c.relevant[argument] {
			continue
		}
		assignment, err := c.assignment(argument)
		if err != nil {
			return err
		}
		if assignment == value {
			*todo = append(*todo, argument)
			return nil
		}
	}
	return ast.ErrASTDomain
}

func (c *SMTRelevantExprContext) pushChildren(expression ast.ASTHandle, todo *[]ast.ASTHandle) error {
	family, err := c.manager.GetFamilyID(expression)
	if err != nil {
		return err
	}
	if family != c.manager.BasicFamilyID() {
		return c.pushAll(expression, todo)
	}
	op, err := c.manager.GetDeclKind(expression)
	if err != nil {
		return err
	}
	switch op {
	case ast.ASTOpAnd, ast.ASTOpOr:
		value, err := c.assignment(expression)
		if err != nil || value == lbool.Undef {
			if err != nil {
				return err
			}
			return ast.ErrASTDomain
		}
		if op == ast.ASTOpAnd && value == lbool.True || op == ast.ASTOpOr && value == lbool.False {
			return c.pushAll(expression, todo)
		}
		return c.pushRelevantChild(expression, value, todo)
	case ast.ASTOpIte:
		arguments, err := c.arguments(expression)
		if err != nil || len(arguments) != 3 {
			if err != nil {
				return err
			}
			return ast.ErrASTDomain
		}
		condition := arguments[0]
		value, err := c.assignment(condition)
		if err != nil || value == lbool.Undef {
			if err != nil {
				return err
			}
			return ast.ErrASTDomain
		}
		branch := arguments[1]
		if value == lbool.False {
			branch = arguments[2]
		}
		if !c.relevant[condition] || !c.relevant[branch] {
			return ast.ErrASTDomain
		}
		if _, found := c.cache[condition]; !found {
			*todo = append(*todo, condition)
		}
		if _, found := c.cache[branch]; !found {
			*todo = append(*todo, branch)
		}
		return nil
	default:
		return c.pushAll(expression, todo)
	}
}

func (c *SMTRelevantExprContext) Process(expression ast.ASTHandle, visit func(ast.ASTHandle) error) error {
	if c == nil || c.manager == nil || c.cache == nil || visit == nil {
		return ast.ErrASTHandle
	}
	if !c.relevant[expression] {
		return ast.ErrASTDomain
	}
	if _, found := c.cache[expression]; found {
		return nil
	}
	todo := []ast.ASTHandle{expression}
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if !c.relevant[current] {
			return ast.ErrASTDomain
		}
		if _, found := c.cache[current]; found {
			continue
		}
		if err := visit(current); err != nil {
			return err
		}
		c.cache[current] = struct{}{}
		kind, err := c.manager.GetKind(current)
		if err != nil {
			return err
		}
		if kind == ast.ASTApp {
			if err := c.pushChildren(current, &todo); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *SMTRelevantExprContext) CollectLabelLiterals(expression ast.ASTHandle, out *[]symbol.Symbol) error {
	if out == nil {
		return ast.ErrASTDomain
	}
	return c.Process(expression, func(current ast.ASTHandle) error {
		value, err := c.assignment(current)
		if err != nil || value != lbool.True {
			return err
		}
		var names []symbol.Symbol
		matched, err := c.manager.IsLabelLit(current, &names)
		if err == nil && matched {
			*out = append(*out, names...)
		}
		return err
	})
}

func (c *SMTRelevantExprContext) CollectLabels(expression ast.ASTHandle, out *[]symbol.Symbol) error {
	if out == nil {
		return ast.ErrASTDomain
	}
	return c.Process(expression, func(current ast.ASTHandle) error {
		var names []symbol.Symbol
		var positive bool
		matched, err := c.manager.IsLabel(current, &positive, &names)
		if err != nil || !matched {
			return err
		}
		value, err := c.assignment(current)
		if err == nil && (positive && value == lbool.True || !positive && value == lbool.False) {
			*out = append(*out, names...)
		}
		return err
	})
}
