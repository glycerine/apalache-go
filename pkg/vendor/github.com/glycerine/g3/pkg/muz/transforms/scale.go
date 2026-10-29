package transforms

// Source: src/muz/transforms/dl_mk_scale.cpp/.h homogeneous scaling of
// linear Real Horn clauses and predicate-signature mapping.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type ScalePlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	oldToNew map[ast.ASTHandle]ast.ASTHandle
	closed   bool
}

func NewScalePlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *ScalePlugin {
	return &ScalePlugin{manager: manager, rules: rules, priority: priority, oldToNew: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (p *ScalePlugin) Priority() uint32            { return p.priority }
func (p *ScalePlugin) CanDestratifyNegation() bool { return false }
func (p *ScalePlugin) Cancel()                     {}

func (p *ScalePlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.oldToNew))
	for old, fresh := range p.oldToNew {
		result[old] = fresh
	}
	return result
}

type scaleRuleState struct {
	plugin   *ScalePlugin
	sigma    uint32
	realSort ast.ASTHandle
	pins     []ast.ASTRef
	eqs      []ast.ASTHandle
	cache    map[ast.ASTHandle]ast.ASTHandle
}

func (s *scaleRuleState) pin(handle ast.ASTHandle) (ast.ASTHandle, error) {
	reference, err := s.plugin.manager.NewRef(handle)
	if err == nil {
		s.pins = append(s.pins, reference)
	}
	return handle, err
}

func (s *scaleRuleState) release() (result error) {
	for i := len(s.pins) - 1; i >= 0; i-- {
		result = errors.Join(result, s.plugin.manager.DelRef(&s.pins[i]))
	}
	return result
}

func (s *scaleRuleState) predicate(source, destination *base.RuleSet, application ast.ASTHandle) (ast.ASTHandle, error) {
	manager := s.plugin.manager
	old, err := manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh := s.plugin.oldToNew[old]
	if fresh.IsNull() {
		arity, getErr := manager.GetArity(old)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		domain := make([]ast.ASTHandle, arity+1)
		for i := uint32(0); i < arity; i++ {
			domain[i], getErr = manager.GetDomain(old, i)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
		}
		domain[arity] = s.realSort
		name, getErr := manager.GetASTName(old)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		rangeSort, getErr := manager.GetRange(old)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		fresh, getErr = manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = s.plugin.rules.RegisterPredicate(fresh); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = destination.InheritPredicate(source, old, fresh); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		s.plugin.oldToNew[old] = fresh
	}
	count, err := manager.GetNumArgs(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, 0, count+1)
	sigma, err := manager.MkVar(s.sigma, s.realSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	for i := uint32(0); i < count; i++ {
		argument, getErr := manager.GetArg(application, i)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		text, _, numeral, getErr := manager.ArithmeticNumeralText(argument)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if numeral {
			switch text {
			case "0":
			case "1":
				argument = sigma
			default:
				argument, getErr = manager.MkVar(s.sigma+1+uint32(len(s.eqs)), s.realSort)
				if getErr != nil {
					return ast.ASTHandle{}, getErr
				}
				original, getOriginalErr := manager.GetArg(application, i)
				if getOriginalErr != nil {
					return ast.ASTHandle{}, getOriginalErr
				}
				product, getErr := manager.MkArithmeticMul(original, sigma)
				if getErr != nil {
					return ast.ASTHandle{}, getErr
				}
				if product, getErr = s.pin(product); getErr != nil {
					return ast.ASTHandle{}, getErr
				}
				equality, getErr := manager.MkEq(argument, product)
				if getErr != nil {
					return ast.ASTHandle{}, getErr
				}
				if equality, getErr = s.pin(equality); getErr != nil {
					return ast.ASTHandle{}, getErr
				}
				s.eqs = append(s.eqs, equality)
			}
		}
		arguments = append(arguments, argument)
	}
	arguments = append(arguments, sigma)
	result, err := manager.MkApp(fresh, arguments)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return s.pin(result)
}

func (s *scaleRuleState) linearize(expression ast.ASTHandle) (ast.ASTHandle, error) {
	if cached := s.cache[expression]; !cached.IsNull() {
		return cached, nil
	}
	manager := s.plugin.manager
	kind, err := manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return expression, err
	}
	family, err := manager.GetFamilyID(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arithmetic, err := manager.ArithmeticFamilyID()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	op, err := manager.GetDeclKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var result ast.ASTHandle
	if family == manager.BasicFamilyID() || family == arithmetic && (op == 6 || op == 7 || op == 2 || op == 3 || op == 4 || op == 5) {
		declaration, getErr := manager.GetDecl(expression)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		arguments, getErr := manager.AppArgumentView(expression)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		rewritten := make([]ast.ASTHandle, arguments.Count())
		for i := uint32(0); i < arguments.Count(); i++ {
			argument, argumentErr := arguments.At(i)
			if argumentErr == nil {
				rewritten[i], argumentErr = s.linearize(argument)
			}
			if argumentErr != nil {
				return ast.ASTHandle{}, argumentErr
			}
		}
		result, err = manager.RewriterMakeApp(declaration, ast.SliceArguments(rewritten))
	} else {
		_, _, numeral, numeralErr := manager.ArithmeticNumeralText(expression)
		if numeralErr != nil {
			return ast.ASTHandle{}, numeralErr
		}
		if numeral {
			sigma, makeErr := manager.MkVar(s.sigma, s.realSort)
			if makeErr != nil {
				return ast.ASTHandle{}, makeErr
			}
			result, err = manager.MkArithmeticMul(sigma, expression)
		} else {
			result = expression
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if result != expression {
		if result, err = s.pin(result); err != nil {
			return ast.ASTHandle{}, err
		}
	}
	s.cache[expression] = result
	return result, nil
}

func (p *ScalePlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil {
		return nil, base.ErrRuleTransformerOwner
	}
	result, err := base.NewRuleSet(p.rules)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			resultErr = errors.Join(resultErr, result.Close())
		}
	}()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	realOne, err := p.manager.ArithmeticRealNumeralText("1")
	if err != nil {
		return nil, err
	}
	realSort, err := p.manager.GetSort(realOne)
	if err != nil {
		return nil, err
	}
	zero, err := p.manager.ArithmeticRealNumeralText("0")
	if err != nil {
		return nil, err
	}
	arithmetic, err := p.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		state := &scaleRuleState{plugin: p, realSort: realSort, cache: make(map[ast.ASTHandle]ast.ASTHandle)}
		state.sigma, getErr = ruleVariableCount(p.manager, rule)
		var replacement *base.Rule
		if getErr == nil {
			uninterpreted, sizeErr := rule.UninterpretedTailSize()
			total, totalErr := rule.TailSize()
			getErr = errors.Join(sizeErr, totalErr)
			tails := make([]ast.ASTHandle, 0, total+2)
			for j := 0; getErr == nil && j < uninterpreted; j++ {
				tail, _, tailErr := rule.Tail(j)
				if tailErr == nil {
					tail, tailErr = state.predicate(source, result, tail)
				}
				getErr = tailErr
				tails = append(tails, tail)
			}
			for j := uninterpreted; getErr == nil && j < total; j++ {
				tail, _, tailErr := rule.Tail(j)
				if tailErr == nil {
					tail, tailErr = state.linearize(tail)
				}
				getErr = tailErr
				tails = append(tails, tail)
			}
			var head ast.ASTHandle
			if getErr == nil {
				head, getErr = rule.Head()
			}
			if getErr == nil {
				head, getErr = state.predicate(source, result, head)
			}
			tails = append(tails, state.eqs...)
			if getErr == nil {
				sigma, makeErr := p.manager.MkVar(state.sigma, realSort)
				var positive ast.ASTHandle
				if makeErr == nil {
					positive, makeErr = p.manager.MkFamilyApp(arithmetic, 5, nil, []ast.ASTHandle{sigma, zero})
				}
				if makeErr == nil {
					positive, makeErr = state.pin(positive)
				}
				getErr = makeErr
				tails = append(tails, positive)
			}
			name, nameErr := rule.Name()
			getErr = errors.Join(getErr, nameErr)
			if getErr == nil {
				replacement, getErr = p.rules.NewRule(head, tails, make([]bool, len(tails)), name)
			}
			if getErr == nil {
				getErr = result.Add(replacement)
			}
		}
		getErr = errors.Join(getErr, state.release())
		if replacement != nil {
			getErr = errors.Join(getErr, replacement.Close())
		}
		getErr = errors.Join(getErr, rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	keep = true
	return result, nil
}

func (p *ScalePlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.oldToNew = nil
	return nil
}
