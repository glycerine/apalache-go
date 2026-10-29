package transforms

// Source: src/muz/transforms/dl_mk_loop_counter.cpp/.h loop-counter
// augmentation and reversion used by Karr invariant discovery.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

type LoopCounterPlugin struct {
	manager  *ast.ASTManager
	rules    *base.RuleManager
	priority uint32
	oldToNew map[ast.ASTHandle]ast.ASTHandle
	newToOld map[ast.ASTHandle]ast.ASTHandle
	closed   bool
}

func NewLoopCounterPlugin(manager *ast.ASTManager, rules *base.RuleManager, priority uint32) *LoopCounterPlugin {
	return &LoopCounterPlugin{manager: manager, rules: rules, priority: priority, oldToNew: make(map[ast.ASTHandle]ast.ASTHandle), newToOld: make(map[ast.ASTHandle]ast.ASTHandle)}
}

func (p *LoopCounterPlugin) Priority() uint32            { return p.priority }
func (p *LoopCounterPlugin) CanDestratifyNegation() bool { return false }
func (p *LoopCounterPlugin) Cancel()                     {}

func (p *LoopCounterPlugin) addArgument(source, destination *base.RuleSet, application ast.ASTHandle, index uint32, integerSort ast.ASTHandle) (ast.ASTHandle, error) {
	old, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	fresh := p.oldToNew[old]
	if fresh.IsNull() {
		arity, getErr := p.manager.GetArity(old)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		domain := make([]ast.ASTHandle, arity+1)
		for i := uint32(0); i < arity; i++ {
			domain[i], getErr = p.manager.GetDomain(old, i)
			if getErr != nil {
				return ast.ASTHandle{}, getErr
			}
		}
		domain[arity] = integerSort
		name, getErr := p.manager.GetASTName(old)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		rangeSort, getErr := p.manager.GetRange(old)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		fresh, getErr = p.manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = p.rules.RegisterPredicate(fresh); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		if getErr = destination.InheritPredicate(source, old, fresh); getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		p.oldToNew[old], p.newToOld[fresh] = fresh, old
	}
	arguments, err := p.manager.AppArgumentView(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	values := make([]ast.ASTHandle, 0, arguments.Count()+1)
	for i := uint32(0); i < arguments.Count(); i++ {
		argument, getErr := arguments.At(i)
		if getErr != nil {
			return ast.ASTHandle{}, getErr
		}
		values = append(values, argument)
	}
	counter, err := p.manager.MkVar(index, integerSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	values = append(values, counter)
	return p.manager.MkApp(fresh, values)
}

func (p *LoopCounterPlugin) removeArgument(application ast.ASTHandle) (ast.ASTHandle, error) {
	fresh, err := p.manager.GetDecl(application)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	old := p.newToOld[fresh]
	if old.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	arguments, err := p.manager.AppArgumentView(application)
	if err != nil || arguments.Count() == 0 {
		return ast.ASTHandle{}, errors.Join(err, ast.ErrASTDomain)
	}
	values := make([]ast.ASTHandle, arguments.Count()-1)
	for i := range values {
		values[i], err = arguments.At(uint32(i))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return p.manager.MkApp(old, values)
}

func (p *LoopCounterPlugin) transform(source *base.RuleSet, revert bool) (_ *base.RuleSet, resultErr error) {
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
	zero, err := p.manager.ArithmeticIntegerNumeralText("0")
	if err != nil {
		return nil, err
	}
	integerSort, err := p.manager.GetSort(zero)
	if err != nil {
		return nil, err
	}
	one, err := p.manager.ArithmeticIntegerNumeralText("1")
	if err != nil {
		return nil, err
	}
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		var pins []ast.ASTRef
		pin := func(handle ast.ASTHandle) (ast.ASTHandle, error) {
			reference, pinErr := p.manager.NewRef(handle)
			if pinErr == nil {
				pins = append(pins, reference)
			}
			return handle, pinErr
		}
		release := func() (releaseErr error) {
			for j := len(pins) - 1; j >= 0; j-- {
				releaseErr = errors.Join(releaseErr, p.manager.DelRef(&pins[j]))
			}
			return releaseErr
		}
		uninterpreted, sizeErr := rule.UninterpretedTailSize()
		total, totalErr := rule.TailSize()
		getErr = errors.Join(sizeErr, totalErr)
		counter := uint32(0)
		if getErr == nil && !revert {
			counter, getErr = ruleVariableCount(p.manager, rule)
		}
		tails := make([]ast.ASTHandle, 0, total+1)
		negated := make([]bool, 0, total+1)
		for j := 0; getErr == nil && j < uninterpreted; j++ {
			tail, negative, tailErr := rule.Tail(j)
			if tailErr == nil {
				if revert {
					tail, tailErr = p.removeArgument(tail)
				} else {
					tail, tailErr = p.addArgument(source, result, tail, counter, integerSort)
					counter++
				}
			}
			if tailErr == nil {
				tail, tailErr = pin(tail)
			}
			getErr = tailErr
			tails, negated = append(tails, tail), append(negated, negative)
		}
		for j := uninterpreted; getErr == nil && j < total; j++ {
			tail, _, tailErr := rule.Tail(j)
			getErr = tailErr
			tails, negated = append(tails, tail), append(negated, false)
		}
		var head ast.ASTHandle
		if getErr == nil {
			head, getErr = rule.Head()
		}
		if getErr == nil {
			if revert {
				head, getErr = p.removeArgument(head)
			} else {
				head, getErr = p.addArgument(source, result, head, counter, integerSort)
			}
		}
		if getErr == nil {
			head, getErr = pin(head)
		}
		if getErr == nil && !revert {
			headDeclaration, declarationErr := p.manager.GetDecl(head)
			last, argumentErr := p.manager.GetNumArgs(head)
			found := false
			for j := 0; declarationErr == nil && argumentErr == nil && !found && j < uninterpreted; j++ {
				tailDeclaration, tailErr := p.manager.GetDecl(tails[j])
				if tailErr != nil {
					argumentErr = tailErr
				} else if tailDeclaration == headDeclaration {
					headCounter, x := p.manager.GetArg(head, last-1)
					tailCount, y := p.manager.GetNumArgs(tails[j])
					var tailCounter, successor, equality ast.ASTHandle
					if x == nil && y == nil {
						tailCounter, x = p.manager.GetArg(tails[j], tailCount-1)
					}
					if x == nil {
						successor, x = p.manager.MkArithmeticAdd(tailCounter, one)
					}
					if x == nil {
						successor, x = pin(successor)
					}
					if x == nil {
						equality, x = p.manager.MkEq(headCounter, successor)
					}
					if x == nil {
						equality, x = pin(equality)
					}
					argumentErr = errors.Join(x, y)
					tails, negated = append(tails, equality), append(negated, false)
					found = argumentErr == nil
				}
			}
			getErr = errors.Join(declarationErr, argumentErr)
			if getErr == nil && !found {
				arguments, argumentErr := p.manager.AppArgumentView(head)
				values := make([]ast.ASTHandle, arguments.Count())
				for j := uint32(0); argumentErr == nil && j < arguments.Count(); j++ {
					values[j], argumentErr = arguments.At(j)
				}
				if argumentErr == nil {
					values[len(values)-1] = zero
					head, argumentErr = p.manager.RewriterMakeApp(headDeclaration, ast.SliceArguments(values))
				}
				if argumentErr == nil {
					head, argumentErr = pin(head)
				}
				getErr = argumentErr
			}
		}
		name, nameErr := rule.Name()
		getErr = errors.Join(getErr, nameErr)
		var replacement *base.Rule
		if getErr == nil {
			replacement, getErr = p.rules.NewRule(head, tails, negated, name)
		}
		if getErr == nil {
			getErr = result.Add(replacement)
		}
		if replacement != nil {
			getErr = errors.Join(getErr, replacement.Close())
		}
		getErr = errors.Join(getErr, release(), rule.Close())
		if getErr != nil {
			return nil, getErr
		}
	}
	keep = true
	return result, nil
}

func (p *LoopCounterPlugin) Transform(source *base.RuleSet) (*base.RuleSet, error) {
	p.oldToNew = make(map[ast.ASTHandle]ast.ASTHandle)
	p.newToOld = make(map[ast.ASTHandle]ast.ASTHandle)
	return p.transform(source, false)
}

func (p *LoopCounterPlugin) Revert(source *base.RuleSet) (*base.RuleSet, error) {
	return p.transform(source, true)
}

func (p *LoopCounterPlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.oldToNew))
	for original, derived := range p.oldToNew {
		result[original] = derived
	}
	return result
}

func (p *LoopCounterPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.oldToNew, p.newToOld = nil, nil
	return nil
}
