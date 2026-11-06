package ddnf

// Source: src/muz/ddnf/ddnf.cpp ddnf::imp::pre_process_rule,
// process_todo, process_atomic, and process_eq.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
)

// Preprocessor recognizes the equality fragment consumed by the source DDNF
// compiler and records each supported bit-vector equality as a ternary vector.
// The manager and registry are borrowed; vectors remain owned by their DDNF
// core until that registry is closed.
type Preprocessor struct {
	manager     *ast.ASTManager
	registry    *Registry
	visited     map[ast.ASTHandle]struct{}
	constraints map[ast.ASTHandle]equalityConstraint
	failed      bool
	closed      bool
}

type equalityConstraint struct {
	vector   *rel.TBV
	variable ast.ASTHandle
}

func NewPreprocessor(manager *ast.ASTManager, registry *Registry) (*Preprocessor, error) {
	if manager == nil || registry == nil {
		return nil, ErrCore
	}
	return &Preprocessor{
		manager: manager, registry: registry,
		visited: make(map[ast.ASTHandle]struct{}), constraints: make(map[ast.ASTHandle]equalityConstraint),
	}, nil
}

func (p *Preprocessor) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.visited = nil
	p.constraints = nil
	p.manager = nil
	p.registry = nil
	p.closed = true
	return nil
}

// PreprocessRules clears query-local traversal and equality state, then checks
// every rule in insertion order as source pre_process_rules does.
func (p *Preprocessor) PreprocessRules(rules *base.RuleSet) (bool, error) {
	return p.preprocessRules(rules, nil, nil)
}

// PreprocessRulesWithDiagnostic matches the source rejection path by printing
// the first unsupported rule before returning false. The writer and rule
// manager are borrowed; both must be supplied together or omitted together.
func (p *Preprocessor) PreprocessRulesWithDiagnostic(rules *base.RuleSet, ruleManager *base.RuleManager, out io.Writer) (bool, error) {
	if (ruleManager == nil) != (out == nil) {
		return false, ErrCore
	}
	return p.preprocessRules(rules, ruleManager, out)
}

func (p *Preprocessor) preprocessRules(rules *base.RuleSet, ruleManager *base.RuleManager, diagnostic io.Writer) (bool, error) {
	if p == nil || p.closed || p.manager == nil || p.registry == nil || rules == nil {
		return false, ErrCore
	}
	p.visited = make(map[ast.ASTHandle]struct{})
	p.constraints = make(map[ast.ASTHandle]equalityConstraint)
	p.failed = false
	count, err := rules.Len()
	if err != nil {
		return false, err
	}
	for index := 0; index < count; index++ {
		rule, getErr := rules.Rule(index)
		if getErr != nil {
			return false, getErr
		}
		ok, processErr := p.ProcessRule(rule)
		var diagnosticErr error
		if !ok && processErr == nil && diagnostic != nil {
			diagnosticErr = ruleManager.WriteRuleDisplay(diagnostic, rule)
		}
		closeErr := rule.Close()
		if processErr != nil || !ok || diagnosticErr != nil || closeErr != nil {
			return ok, errors.Join(processErr, diagnosticErr, closeErr)
		}
	}
	return true, nil
}

// ProcessRule checks every interpreted tail. It returns false for quantifiers
// and atoms outside the source DDNF fragment, matching pre_process_rule.
func (p *Preprocessor) ProcessRule(rule *base.Rule) (bool, error) {
	if p == nil || p.closed || p.failed || p.manager == nil || p.registry == nil || rule == nil {
		return false, ErrCore
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return false, err
	}
	total, err := rule.TailSize()
	if err != nil {
		return false, err
	}
	var roots []ast.ASTHandle
	for index := uninterpreted; index < total; index++ {
		tail, _, getErr := rule.Tail(index)
		if getErr != nil {
			return false, getErr
		}
		roots = append(roots, tail)
	}
	return p.processMany(roots)
}

// Constraint returns the borrowed ternary vector recorded for a supported
// bit-vector equality. The vector belongs to the width-specific core.
func (p *Preprocessor) Constraint(expression ast.ASTHandle) (*rel.TBV, bool, error) {
	if p == nil || p.closed {
		return nil, false, ErrCore
	}
	constraint, found := p.constraints[expression]
	return constraint.vector, found, nil
}

func (p *Preprocessor) processMany(roots []ast.ASTHandle) (bool, error) {
	todo := append([]ast.ASTHandle(nil), roots...)
	for len(todo) != 0 {
		last := len(todo) - 1
		expression := todo[last]
		todo = todo[:last]
		if _, found := p.visited[expression]; found {
			continue
		}
		p.visited[expression] = struct{}{}
		kind, err := p.manager.GetKind(expression)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTVar {
			continue
		}
		if kind == ast.ASTQuantifier {
			return false, nil
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		operator, err := p.manager.GetDeclKind(expression)
		if err != nil {
			return false, err
		}
		switch operator {
		case ast.ASTOpAnd, ast.ASTOpOr, ast.ASTOpIff, ast.ASTOpNot, ast.ASTOpImplies:
			count, e := p.manager.GetNumArgs(expression)
			if e != nil {
				return false, e
			}
			for index := count; index != 0; index-- {
				argument, x := p.manager.GetArg(expression, index-1)
				if x != nil {
					return false, x
				}
				todo = append(todo, argument)
			}
			continue
		}
		used, err := ast.NewASTUsedVars(p.manager)
		if err != nil {
			return false, err
		}
		if err = used.Collect(expression); err != nil {
			return false, err
		}
		if used.NumVars() == 0 {
			continue
		}
		ok, err := p.processAtomic(expression)
		if err != nil || !ok {
			if err == nil {
				p.failed = true
			}
			return ok, err
		}
	}
	return true, nil
}

func (p *Preprocessor) processAtomic(expression ast.ASTHandle) (bool, error) {
	operator, err := p.manager.GetDeclKind(expression)
	if err != nil || operator != ast.ASTOpEq {
		return false, err
	}
	left, err := p.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	right, err := p.manager.GetArg(expression, 1)
	if err != nil {
		return false, err
	}
	isBV, err := p.manager.IsBitVector(left)
	if err != nil || !isBV {
		return false, err
	}
	leftKind, err := p.manager.GetKind(left)
	if err != nil {
		return false, err
	}
	rightKind, err := p.manager.GetKind(right)
	if err != nil {
		return false, err
	}
	leftGround, err := p.isGround(left)
	if err != nil {
		return false, err
	}
	rightGround, err := p.isGround(right)
	if err != nil {
		return false, err
	}
	if leftKind == ast.ASTVar && rightGround {
		return p.processEquality(expression, left, right, nil)
	}
	if rightKind == ast.ASTVar && leftGround {
		return p.processEquality(expression, right, left, nil)
	}
	var base ast.ASTHandle
	var low, high uint32
	if extracted, e := p.manager.BitVectorExtractValue(left, &low, &high, &base); e != nil {
		return false, e
	} else if extracted && rightGround {
		if kind, x := p.manager.GetKind(base); x != nil {
			return false, x
		} else if kind == ast.ASTVar {
			return p.processEquality(expression, base, right, &bitRange{high: high, low: low})
		}
	}
	if extracted, e := p.manager.BitVectorExtractValue(right, &low, &high, &base); e != nil {
		return false, e
	} else if extracted && leftGround {
		if kind, x := p.manager.GetKind(base); x != nil {
			return false, x
		} else if kind == ast.ASTVar {
			return p.processEquality(expression, base, left, &bitRange{high: high, low: low})
		}
	}
	if leftKind == ast.ASTVar && rightKind == ast.ASTVar {
		return true, nil
	}
	return false, nil
}

type bitRange struct{ high, low uint32 }

func (p *Preprocessor) processEquality(expression, variable, numeralExpression ast.ASTHandle, bitRange *bitRange) (bool, error) {
	width, err := p.manager.BitVectorSize(variable)
	if err != nil {
		return false, err
	}
	service := p.manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return false, err
	}
	defer service.Del(&value)
	var ignoredWidth uint32
	isNumeral, err := p.manager.BitVectorNumeralValue(numeralExpression, &value, &ignoredWidth)
	if err != nil || !isNumeral {
		return false, err
	}
	isUint64, err := service.IsUint64(&value)
	if err != nil || !isUint64 {
		return false, err
	}
	unsigned, err := service.Uint64(&value)
	if err != nil {
		return false, err
	}
	high, low := width-1, uint32(0)
	if bitRange != nil {
		high, low = bitRange.high, bitRange.low
	}
	core, err := p.registry.Get(width)
	if err != nil {
		return false, err
	}
	vector, err := core.Manager().AllocateUint64Range(unsigned, high, low)
	if err != nil {
		return false, err
	}
	node, err := core.Insert(vector)
	if err != nil {
		_ = core.Manager().Deallocate(vector)
		return false, err
	}
	if node.Vector() != vector {
		if err = core.Manager().Deallocate(vector); err != nil {
			return false, err
		}
		vector = node.Vector()
	}
	p.constraints[expression] = equalityConstraint{vector: vector, variable: variable}
	return true, nil
}

func (p *Preprocessor) isGround(expression ast.ASTHandle) (bool, error) {
	used, err := ast.NewASTUsedVars(p.manager)
	if err != nil {
		return false, err
	}
	if err = used.Collect(expression); err != nil {
		return false, err
	}
	return used.NumVars() == 0, nil
}
