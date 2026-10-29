package transforms

// Source: src/muz/transforms/dl_mk_bit_blast.cpp/.h retained Horn-rule
// bit blasting, predicate-signature expansion, and predicate mapping.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
	bitblaster "github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type bitBlastExpansion struct {
	astrewriter.DefaultConfiguration
	manager  *ast.ASTManager
	rules    *base.RuleManager
	source   *base.RuleSet
	target   *base.RuleSet
	oldToNew map[ast.ASTHandle]ast.ASTHandle
}

func (c *bitBlastExpansion) ReduceApp(declaration ast.ASTHandle, arguments ast.AppArguments, out, _ ast.ASTRef) (astrewriter.RewriteStatus, error) {
	if arguments.Count() == 0 {
		return astrewriter.RewriteFailed, nil
	}
	flat := make([]ast.ASTHandle, 0)
	for i := uint32(0); i < arguments.Count(); i++ {
		argument, err := arguments.At(i)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		assembled, err := c.manager.IsBitVectorOperation(argument, 56)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		if !assembled {
			return astrewriter.RewriteFailed, nil
		}
		count, err := c.manager.GetNumArgs(argument)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		for j := uint32(0); j < count; j++ {
			bit, getErr := c.manager.GetArg(argument, j)
			if getErr != nil {
				return astrewriter.RewriteFailed, getErr
			}
			flat = append(flat, bit)
		}
	}
	fresh := c.oldToNew[declaration]
	if fresh.IsNull() {
		name, err := c.manager.GetASTName(declaration)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		suffix, err := c.manager.Symbols.Intern("bv")
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		domain := make([]ast.ASTHandle, len(flat))
		for i, bit := range flat {
			domain[i], err = c.manager.GetSort(bit)
			if err != nil {
				return astrewriter.RewriteFailed, err
			}
		}
		rangeSort, err := c.manager.GetRange(declaration)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		fresh, err = c.manager.MkFreshFuncDecl(name, suffix, domain, rangeSort)
		if err != nil {
			return astrewriter.RewriteFailed, err
		}
		if err = c.rules.RegisterPredicate(fresh); err != nil {
			return astrewriter.RewriteFailed, err
		}
		if err = c.target.InheritPredicate(c.source, declaration, fresh); err != nil {
			return astrewriter.RewriteFailed, err
		}
		c.oldToNew[declaration] = fresh
	}
	application, err := c.manager.MkApp(fresh, flat)
	if err != nil {
		return astrewriter.RewriteFailed, err
	}
	if err = c.manager.SetRef(out, application); err != nil {
		return astrewriter.RewriteFailed, err
	}
	return astrewriter.RewriteDone, nil
}

type BitBlastPlugin struct {
	manager    *ast.ASTManager
	rules      *base.RuleManager
	simplifier *InterpTailSimplifierPlugin
	blaster    *bitblaster.BitBlasterRewriter
	expansion  bitBlastExpansion
	priority   uint32
	closed     bool
}

func NewBitBlastPlugin(manager *ast.ASTManager, rules *base.RuleManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, priority uint32) (_ *BitBlastPlugin, resultErr error) {
	params := bitblaster.DefaultRewriterParams()
	params.BlastFull, params.BlastQuant = true, true
	blaster, err := bitblaster.NewBitBlasterRewriter(manager, &params)
	if err != nil {
		return nil, err
	}
	simplifier, err := NewInterpTailSimplifierPlugin(manager, rules, context, globals, priority)
	if err != nil {
		return nil, errors.Join(err, blaster.Close())
	}
	p := &BitBlastPlugin{manager: manager, rules: rules, simplifier: simplifier, blaster: blaster, priority: priority}
	p.expansion.manager, p.expansion.rules = manager, rules
	p.expansion.oldToNew = make(map[ast.ASTHandle]ast.ASTHandle)
	return p, nil
}

func (p *BitBlastPlugin) Priority() uint32            { return p.priority }
func (p *BitBlastPlugin) CanDestratifyNegation() bool { return false }
func (p *BitBlastPlugin) Cancel()                     {}

func (p *BitBlastPlugin) FreshPredicates() map[ast.ASTHandle]ast.ASTHandle {
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(p.expansion.oldToNew))
	for old, fresh := range p.expansion.oldToNew {
		result[old] = fresh
	}
	return result
}

func (p *BitBlastPlugin) Transform(source *base.RuleSet) (_ *base.RuleSet, resultErr error) {
	if p == nil || p.closed || p.manager == nil || p.rules == nil || p.blaster == nil {
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
	if err = result.InheritPredicates(source); err != nil {
		return nil, err
	}
	p.expansion.source, p.expansion.target = source, result
	traversal, err := astrewriter.NewTraversal(p.manager, &p.expansion)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, traversal.Close()) }()
	count, err := source.Len()
	if err != nil {
		return nil, err
	}
	changed := false
	for i := 0; i < count; i++ {
		rule, getErr := source.Rule(i)
		if getErr != nil {
			return nil, getErr
		}
		current, dropped, simplified, transformErr := p.simplifier.transformRule(rule)
		if transformErr == nil && dropped {
			changed = true
		}
		if transformErr == nil && !dropped {
			if current == nil {
				current = rule
			}
			formula, formulaErr := current.ToFormula()
			transformErr = formulaErr
			var blasted, expanded ast.ASTRef
			release := func(reference *ast.ASTRef) {
				if *reference != (ast.ASTRef{}) {
					transformErr = errors.Join(transformErr, p.manager.DelRef(reference))
				}
			}
			if transformErr == nil {
				blasted, transformErr = p.manager.NewRef(ast.ASTHandle{})
			}
			if transformErr == nil {
				var original ast.ASTHandle
				original, transformErr = p.manager.RefGet(formula)
				if transformErr == nil {
					transformErr = p.blaster.Rewrite(original, blasted)
				}
			}
			if transformErr == nil {
				expanded, transformErr = p.manager.NewRef(ast.ASTHandle{})
			}
			if transformErr == nil {
				var blastedHandle ast.ASTHandle
				blastedHandle, transformErr = p.manager.RefGet(blasted)
				if transformErr == nil {
					transformErr = traversal.Rewrite(blastedHandle, expanded)
				}
			}
			if transformErr == nil {
				original, getOriginalErr := p.manager.RefGet(formula)
				final, getFinalErr := p.manager.RefGet(expanded)
				transformErr = errors.Join(getOriginalErr, getFinalErr)
				if transformErr == nil && (simplified || final != original) {
					var replacement *base.Rule
					name, nameErr := rule.Name()
					transformErr = errors.Join(transformErr, nameErr)
					if transformErr == nil {
						replacement, transformErr = p.rules.NewRuleFromFormula(final, name)
					}
					if transformErr == nil {
						transformErr = result.Add(replacement)
					}
					if replacement != nil {
						transformErr = errors.Join(transformErr, replacement.Close())
					}
					changed = true
				} else if transformErr == nil {
					transformErr = result.Add(rule)
				}
			}
			release(&expanded)
			release(&blasted)
			release(&formula)
		}
		if current != nil && current != rule {
			transformErr = errors.Join(transformErr, current.Close())
		}
		transformErr = errors.Join(transformErr, rule.Close())
		if transformErr != nil {
			return nil, transformErr
		}
	}
	if !changed {
		return nil, nil
	}
	keep = true
	return result, nil
}

func (p *BitBlastPlugin) Close() error {
	if p == nil || p.closed {
		return nil
	}
	p.closed = true
	p.expansion.source, p.expansion.target = nil, nil
	p.expansion.oldToNew = nil
	return errors.Join(p.simplifier.Close(), p.blaster.Close())
}
