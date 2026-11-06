package pdr

// Source: context::classifier_proc in src/muz/pdr/pdr_context.cpp:1596-1693.

import "github.com/glycerine/g3/pkg/ast"

type contextRuleClassifier struct {
	manager       *ast.ASTManager
	boolean       ast.ASTHandle
	arithmetic    ast.FamilyID
	seen          map[ast.ASTHandle]bool
	nonBoolean    bool
	hasArithmetic bool
}

func newContextRuleClassifier(manager *ast.ASTManager) (*contextRuleClassifier, error) {
	boolean, err := manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return &contextRuleClassifier{manager: manager, boolean: boolean, arithmetic: arithmetic, seen: make(map[ast.ASTHandle]bool)}, nil
}

// Predicate declarations are Horn relation symbols, not interpreted Boolean
// applications. Only their arguments participate in the source classifier.
func (c *contextRuleClassifier) predicate(application ast.ASTHandle) error {
	count, err := c.manager.GetNumArgs(application)
	if err != nil {
		return err
	}
	for index := uint32(0); index < count; index++ {
		argument, err := c.manager.GetArg(application, index)
		if err != nil {
			return err
		}
		if err = c.expression(argument); err != nil {
			return err
		}
	}
	return nil
}

func (c *contextRuleClassifier) expression(root ast.ASTHandle) error {
	stack := []ast.ASTHandle{root}
	for len(stack) != 0 {
		expression := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if c.seen[expression] {
			continue
		}
		c.seen[expression] = true
		sort, err := c.manager.GetSort(expression)
		if err != nil {
			return err
		}
		c.nonBoolean = c.nonBoolean || sort != c.boolean
		family, err := c.manager.GetFamilyID(sort)
		if err != nil {
			return err
		}
		if family == c.arithmetic {
			kind, err := c.manager.GetDeclKind(sort)
			if err != nil {
				return err
			}
			c.hasArithmetic = c.hasArithmetic || kind == 0 || kind == 1
		}
		kind, err := c.manager.GetKind(expression)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTVar:
		case ast.ASTApp:
			count, err := c.manager.GetNumArgs(expression)
			if err != nil {
				return err
			}
			family, err := c.manager.GetFamilyID(expression)
			if err != nil {
				return err
			}
			c.nonBoolean = c.nonBoolean || count > 0 && family != c.manager.BasicFamilyID()
			for index := uint32(0); index < count; index++ {
				argument, err := c.manager.GetArg(expression, index)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			c.nonBoolean = true
			body, err := c.manager.GetQuantifierBody(expression)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			count, err := c.manager.GetNumPatterns(expression)
			if err != nil {
				return err
			}
			for index := uint32(0); index < count; index++ {
				pattern, err := c.manager.GetPattern(expression, index)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
			count, err = c.manager.GetNumNoPatterns(expression)
			if err != nil {
				return err
			}
			for index := uint32(0); index < count; index++ {
				pattern, err := c.manager.GetNoPattern(expression, index)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
		default:
			return ast.ErrASTDomain
		}
	}
	return nil
}
