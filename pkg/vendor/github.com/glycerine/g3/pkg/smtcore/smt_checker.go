package smtcore

// Source: src/smt/smt_checker.h and src/smt/smt_checker.cpp.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// SMTCheckerContext is the concrete read-side context state used by SMTChecker.
// The complete SMT context can own this component and update it as terms are
// internalized, assigned, and merged.
type SMTCheckerContext struct {
	manager       *ast.ASTManager
	relevancy     *SMTRelevancyPropagator
	relevant      map[ast.ASTHandle]bool
	assignments   map[ast.ASTHandle]lbool.LBool
	enodes        map[ast.ASTHandle]*SMTEnode
	byDeclaration map[ast.ASTHandle][]*SMTEnode
	disequalities map[smtEnodePairKey]struct{}
}

// Source context::is_relevant includes all expressions when filtering is
// disabled. Standalone read-side contexts retain their explicitly marked view.
func (c *SMTCheckerContext) isRelevant(expression ast.ASTHandle) bool {
	if c.relevancy != nil {
		return c.relevancy.IsRelevant(expression)
	}
	return c.relevant[expression]
}

func NewSMTCheckerContext(manager *ast.ASTManager) (*SMTCheckerContext, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTCheckerContext{
		manager: manager, relevant: make(map[ast.ASTHandle]bool),
		assignments: make(map[ast.ASTHandle]lbool.LBool), enodes: make(map[ast.ASTHandle]*SMTEnode),
		byDeclaration: make(map[ast.ASTHandle][]*SMTEnode), disequalities: make(map[smtEnodePairKey]struct{}),
	}, nil
}

func (c *SMTCheckerContext) SetRelevant(expression ast.ASTHandle, relevant bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if _, err := c.manager.GetKind(expression); err != nil {
		return err
	}
	c.relevant[expression] = relevant
	return nil
}

func (c *SMTCheckerContext) SetAssignment(expression ast.ASTHandle, value lbool.LBool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if value != lbool.False && value != lbool.Undef && value != lbool.True {
		return ast.ErrASTDomain
	}
	if _, err := c.manager.GetKind(expression); err != nil {
		return err
	}
	c.assignments[expression] = value
	return nil
}

func (c *SMTCheckerContext) RegisterEnode(node *SMTEnode, relevant bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if node == nil || node.manager != c.manager {
		return ast.ErrASTDomain
	}
	if err := node.valid(); err != nil {
		return err
	}
	if prior := c.enodes[node.owner]; prior != nil && prior != node {
		return ast.ErrASTDomain
	}
	if c.enodes[node.owner] == nil {
		c.enodes[node.owner] = node
		c.byDeclaration[node.declaration] = append(c.byDeclaration[node.declaration], node)
	}
	c.relevant[node.owner] = relevant
	return nil
}

func smtCheckerPair(first, second *SMTEnode) (smtEnodePairKey, error) {
	if first == nil || second == nil || first.manager == nil || first.manager != second.manager {
		return smtEnodePairKey{}, ast.ErrASTDomain
	}
	first, second = first.root, second.root
	if first == nil || second == nil || first == second {
		return smtEnodePairKey{}, ast.ErrASTDomain
	}
	firstID, err := first.OwnerID()
	if err != nil {
		return smtEnodePairKey{}, err
	}
	secondID, err := second.OwnerID()
	if err != nil {
		return smtEnodePairKey{}, err
	}
	if firstID > secondID {
		first, second = second, first
	}
	return smtEnodePairKey{first: first, second: second}, nil
}

func (c *SMTCheckerContext) SetDisequal(first, second *SMTEnode, disequal bool) error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	key, err := smtCheckerPair(first, second)
	if err != nil {
		return err
	}
	if key.first.manager != c.manager {
		return ast.ErrASTDomain
	}
	if disequal {
		c.disequalities[key] = struct{}{}
	} else {
		delete(c.disequalities, key)
	}
	return nil
}

func (c *SMTCheckerContext) isDisequal(first, second *SMTEnode) bool {
	key, err := smtCheckerPair(first, second)
	if err != nil {
		return false
	}
	_, found := c.disequalities[key]
	return found
}

type smtCheckerTruthKey struct {
	expression ast.ASTHandle
	value      bool
}

type SMTChecker struct {
	context    *SMTCheckerContext
	truthCache map[smtCheckerTruthKey]bool
	enodeCache map[ast.ASTHandle]*SMTEnode
	bindings   []*SMTEnode
}

func NewSMTChecker(context *SMTCheckerContext) (*SMTChecker, error) {
	if context == nil || context.manager == nil {
		return nil, ast.ErrASTHandle
	}
	return &SMTChecker{context: context}, nil
}

func (c *SMTChecker) clear() {
	c.truthCache = nil
	c.enodeCache = nil
	c.bindings = nil
}

func (c *SMTChecker) arguments(expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := c.context.manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	arguments := make([]ast.ASTHandle, n)
	for i := uint32(0); i < n; i++ {
		arguments[i], err = c.context.manager.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
	}
	return arguments, nil
}

func (c *SMTChecker) allArgs(expression ast.ASTHandle, value bool) (bool, error) {
	arguments, err := c.arguments(expression)
	if err != nil {
		return false, err
	}
	for _, argument := range arguments {
		result, err := c.check(argument, value)
		if err != nil || !result {
			return result, err
		}
	}
	return true, nil
}

func (c *SMTChecker) anyArg(expression ast.ASTHandle, value bool) (bool, error) {
	arguments, err := c.arguments(expression)
	if err != nil {
		return false, err
	}
	for _, argument := range arguments {
		result, err := c.check(argument, value)
		if err != nil || result {
			return result, err
		}
	}
	return false, nil
}

func (c *SMTChecker) congruentEnode(expression ast.ASTHandle, arguments []*SMTEnode) (*SMTEnode, error) {
	declaration, err := c.context.manager.GetDecl(expression)
	if err != nil {
		return nil, err
	}
	for _, candidate := range c.context.byDeclaration[declaration] {
		if len(candidate.arguments) != len(arguments) {
			continue
		}
		matches := true
		for i := range arguments {
			if candidate.arguments[i].root != arguments[i].root {
				matches = false
				break
			}
		}
		if !matches && candidate.commutative && len(arguments) == 2 {
			matches = candidate.arguments[0].root == arguments[1].root && candidate.arguments[1].root == arguments[0].root
		}
		if matches && c.context.isRelevant(candidate.owner) {
			return candidate, nil
		}
	}
	return nil, nil
}

func (c *SMTChecker) enodeEqualTo(expression ast.ASTHandle) (*SMTEnode, error) {
	if node, found := c.enodeCache[expression]; found {
		return node, nil
	}
	kind, err := c.context.manager.GetKind(expression)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTVar {
		index, err := c.context.manager.GetVarIndex(expression)
		if err != nil || int(index) >= len(c.bindings) {
			return nil, err
		}
		return c.bindings[len(c.bindings)-int(index)-1], nil
	}
	if node := c.context.enodes[expression]; node != nil && c.context.isRelevant(expression) {
		return node, nil
	}
	if kind != ast.ASTApp {
		return nil, nil
	}
	n, err := c.context.manager.GetNumArgs(expression)
	if err != nil || n == 0 {
		return nil, err
	}
	arguments := make([]*SMTEnode, n)
	for i := uint32(0); i < n; i++ {
		argument, err := c.context.manager.GetArg(expression, i)
		if err != nil {
			return nil, err
		}
		arguments[i], err = c.enodeEqualTo(argument)
		if err != nil || arguments[i] == nil {
			return nil, err
		}
	}
	node, err := c.congruentEnode(expression, arguments)
	if err == nil {
		c.enodeCache[expression] = node
	}
	return node, err
}

func (c *SMTChecker) checkCore(expression ast.ASTHandle, value bool) (bool, error) {
	manager := c.context.manager
	sort, err := manager.GetSort(expression)
	if err != nil {
		return false, err
	}
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return false, err
	}
	if sort != boolean {
		return false, ast.ErrASTDomain
	}
	if assignment, found := c.context.assignments[expression]; found && c.context.isRelevant(expression) {
		return assignment != lbool.Undef && value == (assignment == lbool.True), nil
	}
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return false, err
	}
	if family == manager.BasicFamilyID() {
		op, err := manager.GetDeclKind(expression)
		if err != nil {
			return false, err
		}
		arguments, err := c.arguments(expression)
		if err != nil {
			return false, err
		}
		switch op {
		case ast.ASTOpTrue:
			return value, nil
		case ast.ASTOpFalse:
			return !value, nil
		case ast.ASTOpNot:
			if len(arguments) != 1 {
				return false, ast.ErrASTDomain
			}
			return c.check(arguments[0], !value)
		case ast.ASTOpOr:
			if value {
				return c.anyArg(expression, true)
			}
			return c.allArgs(expression, false)
		case ast.ASTOpAnd:
			if value {
				return c.allArgs(expression, true)
			}
			return c.anyArg(expression, false)
		case ast.ASTOpIff:
			if len(arguments) != 2 {
				return false, ast.ErrASTDomain
			}
			leftTrue, err := c.check(arguments[0], true)
			if err != nil {
				return false, err
			}
			if leftTrue {
				return c.check(arguments[1], value)
			}
			leftFalse, err := c.check(arguments[0], false)
			if err != nil || !leftFalse {
				return false, err
			}
			return c.check(arguments[1], !value)
		case ast.ASTOpIte:
			if len(arguments) != 3 {
				return false, ast.ErrASTDomain
			}
			if assignment, found := c.context.assignments[arguments[0]]; found && c.context.isRelevant(arguments[0]) {
				switch assignment {
				case lbool.False:
					return c.check(arguments[2], value)
				case lbool.Undef:
					return false, nil
				case lbool.True:
					return c.check(arguments[1], value)
				}
			}
			thenResult, err := c.check(arguments[1], value)
			if err != nil || !thenResult {
				return false, err
			}
			return c.check(arguments[2], value)
		case ast.ASTOpEq:
			if len(arguments) != 2 {
				return false, ast.ErrASTDomain
			}
			left, err := c.enodeEqualTo(arguments[0])
			if err != nil {
				return false, err
			}
			right, err := c.enodeEqualTo(arguments[1])
			if err != nil || left == nil || right == nil || !c.context.isRelevant(left.owner) || !c.context.isRelevant(right.owner) {
				return false, err
			}
			if value {
				return left.root == right.root, nil
			}
			return c.context.isDisequal(left, right), nil
		}
	}
	node, err := c.enodeEqualTo(expression)
	if err != nil || node == nil || !node.boolean || !c.context.isRelevant(node.owner) {
		return false, err
	}
	assignment, found := c.context.assignments[node.owner]
	return found && assignment != lbool.Undef && value == (assignment == lbool.True), nil
}

func (c *SMTChecker) check(expression ast.ASTHandle, value bool) (bool, error) {
	key := smtCheckerTruthKey{expression: expression, value: value}
	if result, found := c.truthCache[key]; found {
		return result, nil
	}
	result, err := c.checkCore(expression, value)
	if err == nil {
		c.truthCache[key] = result
	}
	return result, err
}

func (c *SMTChecker) run(expression ast.ASTHandle, value bool, bindings []*SMTEnode) (bool, error) {
	if c == nil || c.context == nil || c.context.manager == nil {
		return false, ast.ErrASTHandle
	}
	for _, binding := range bindings {
		if binding == nil || binding.manager != c.context.manager {
			return false, ast.ErrASTDomain
		}
	}
	c.truthCache = make(map[smtCheckerTruthKey]bool)
	c.enodeCache = make(map[ast.ASTHandle]*SMTEnode)
	c.bindings = bindings
	defer c.clear()
	return c.check(expression, value)
}

func (c *SMTChecker) IsSatisfiable(expression ast.ASTHandle, bindings []*SMTEnode) (bool, error) {
	return c.run(expression, true, bindings)
}

func (c *SMTChecker) IsUnsatisfiable(expression ast.ASTHandle, bindings []*SMTEnode) (bool, error) {
	return c.run(expression, false, bindings)
}
