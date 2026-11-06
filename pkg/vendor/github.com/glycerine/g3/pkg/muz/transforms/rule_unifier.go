package transforms

// Source: src/muz/transforms/dl_mk_rule_inliner.cpp/.h, rule_unifier.
// The same rule unifier is used by BMC's proof reconstruction.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/substitution"
	"github.com/glycerine/g3/pkg/muz/base"
)

// RuleUnifier keeps the two variable banks produced by the most recent rule
// unification. Rule resolution and normalization are separate source stages.
type RuleUnifier struct {
	manager   *ast.ASTManager
	subst     *substitution.Substitution
	unifier   *substitution.Unifier
	variables uint32
	ready     bool
	closing   bool
}

func NewRuleUnifier(manager *ast.ASTManager) (*RuleUnifier, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	subst, err := substitution.New(manager)
	if err != nil {
		return nil, err
	}
	unifier, err := substitution.NewUnifier(manager)
	if err != nil {
		return nil, errors.Join(err, subst.Close())
	}
	return &RuleUnifier{manager: manager, subst: subst, unifier: unifier}, nil
}

// UnifyRules unifies the selected target tail with the source head, using
// distinct variable banks for the two rules.
func (u *RuleUnifier) UnifyRules(target *base.Rule, tailIndex int, source *base.Rule) (bool, error) {
	if u == nil || u.manager == nil || u.closing || target == nil || source == nil {
		return false, ast.ErrASTHandle
	}
	u.ready = false
	tail, _, err := target.Tail(tailIndex)
	if err != nil {
		return false, err
	}
	head, err := source.Head()
	if err != nil {
		return false, err
	}
	left, err := ruleVariableCount(u.manager, target)
	if err != nil {
		return false, err
	}
	right, err := ruleVariableCount(u.manager, source)
	if err != nil {
		return false, err
	}
	if right > left {
		left = right
	}
	replacement, err := substitution.New(u.manager)
	if err != nil {
		return false, err
	}
	if err = u.subst.Close(); err != nil {
		return false, errors.Join(err, replacement.Close())
	}
	u.subst = replacement
	if err = u.subst.Reserve(2, left); err != nil {
		return false, err
	}
	unified, err := u.unifier.UnifyPair(tail, head, u.subst, true)
	if err != nil || !unified {
		return false, err
	}
	u.variables, u.ready = left, true
	return true, nil
}

// RuleSubstitution returns owning AST references for the selected rule's
// variable bank. The caller releases each reference through the AST manager.
func (u *RuleUnifier) RuleSubstitution(rule *base.Rule, isTarget bool) ([]ast.ASTRef, error) {
	if u == nil || u.manager == nil || u.closing || rule == nil || !u.ready {
		return nil, ast.ErrASTHandle
	}
	var offset uint32
	if !isTarget {
		offset = 1
	}
	_, refs, err := ruleProofSubstitution(u.manager, rule, offset, u.variables, u.subst)
	return refs, err
}

// ResolveRaw applies the current unifier, removes duplicate tails, and
// normalizes variables. Unbound-variable processing and interpreted-tail
// simplification are separate.
func (u *RuleUnifier) ResolveRaw(rules *base.RuleManager, target *base.Rule, tailIndex int, source *base.Rule) (_ *base.Rule, resultErr error) {
	rule, renaming, err := u.resolveRaw(rules, target, tailIndex, source, false)
	if len(renaming) != 0 {
		err = errors.Join(err, releaseTransformRefs(u.manager, renaming))
	}
	return rule, err
}

// ResolveRawWithRenaming also returns the variable-index compression used
// when constructing the result. Each nonempty reference belongs to the caller.
func (u *RuleUnifier) ResolveRawWithRenaming(rules *base.RuleManager, target *base.Rule, tailIndex int, source *base.Rule) (*base.Rule, []ast.ASTRef, error) {
	return u.resolveRaw(rules, target, tailIndex, source, true)
}

func (u *RuleUnifier) resolveRaw(rules *base.RuleManager, target *base.Rule, tailIndex int, source *base.Rule, wantRenaming bool) (result *base.Rule, renaming []ast.ASTRef, resultErr error) {
	if u == nil || u.manager == nil || u.closing || !u.ready || rules == nil || target == nil || source == nil {
		return nil, nil, ast.ErrASTHandle
	}
	defer func() {
		if resultErr != nil {
			if result != nil {
				resultErr = errors.Join(resultErr, result.Close())
				result = nil
			}
			resultErr = errors.Join(resultErr, releaseTransformRefs(u.manager, renaming))
			renaming = nil
		}
	}()
	deltas := []uint32{0, u.variables}
	var references []ast.ASTRef
	defer func() { resultErr = errors.Join(resultErr, releaseTransformRefs(u.manager, references)) }()
	apply := func(expression ast.ASTHandle, offset uint32) (ast.ASTHandle, error) {
		ref, applyErr := u.manager.NewRef(ast.ASTHandle{})
		if applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		references = append(references, ref)
		if applyErr = u.subst.Apply(2, deltas, substitution.NewExprOffset(expression, offset), ref); applyErr != nil {
			return ast.ASTHandle{}, applyErr
		}
		return u.manager.RefGet(ref)
	}
	targetHead, err := target.Head()
	if err != nil {
		return nil, nil, err
	}
	newHead, err := apply(targetHead, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("muz/transforms: apply inline target head: %w", err)
	}
	var tails []ast.ASTHandle
	var negatives []bool
	appendRule := func(rule *base.Rule, offset uint32, skip int) error {
		count, appendErr := rule.TailSize()
		if appendErr != nil {
			return appendErr
		}
		for index := 0; index < count; index++ {
			if index == skip {
				continue
			}
			expression, negative, getErr := rule.Tail(index)
			if getErr != nil {
				return getErr
			}
			updated, getErr := apply(expression, offset)
			if getErr != nil {
				return getErr
			}
			tails = append(tails, updated)
			negatives = append(negatives, negative)
		}
		return nil
	}
	if err = appendRule(target, 0, tailIndex); err != nil {
		return nil, nil, fmt.Errorf("muz/transforms: apply inline target tail: %w", err)
	}
	if err = appendRule(source, 1, -1); err != nil {
		return nil, nil, fmt.Errorf("muz/transforms: apply inline source tail: %w", err)
	}
	tails, negatives, err = base.RemoveDuplicateRuleTails(tails, negatives)
	if err != nil {
		return nil, nil, fmt.Errorf("muz/transforms: remove duplicate inline tails: %w", err)
	}
	if wantRenaming {
		used, e := ast.NewASTUsedVars(u.manager)
		if e != nil {
			return nil, nil, e
		}
		if e = used.Process(newHead); e != nil {
			return nil, nil, e
		}
		for _, tail := range tails {
			if e = used.Process(tail); e != nil {
				return nil, nil, e
			}
		}
		renaming = make([]ast.ASTRef, used.MaxFoundVarIndexPlusOne())
		var next uint32
		for i := range renaming {
			sort := used.Contains(uint32(i))
			if sort.IsNull() {
				continue
			}
			variable, e := u.manager.MkVar(next, sort)
			if e != nil {
				return nil, renaming, e
			}
			renaming[i], e = u.manager.NewRef(variable)
			if e != nil {
				return nil, renaming, e
			}
			next++
		}
	}
	targetName, err := target.NameText()
	if err != nil {
		return nil, renaming, err
	}
	sourceName, err := source.NameText()
	if err != nil {
		return nil, renaming, err
	}
	resolved, err := rules.NewRule(newHead, tails, negatives, targetName+";"+sourceName)
	if err != nil {
		return nil, renaming, fmt.Errorf("muz/transforms: construct resolved rule: %w", err)
	}
	if err = resolved.SetAccountingParentRule(target); err != nil {
		return nil, renaming, errors.Join(fmt.Errorf("muz/transforms: account resolved rule to target: %w", err), resolved.Close())
	}
	return resolved, renaming, nil
}

func (u *RuleUnifier) Close() error {
	if u == nil || u.manager == nil {
		return ast.ErrASTHandle
	}
	u.closing = true
	u.ready = false
	if u.unifier != nil {
		if err := u.unifier.Close(); err != nil {
			return err
		}
		u.unifier = nil
	}
	if u.subst != nil {
		if err := u.subst.Close(); err != nil {
			return err
		}
		u.subst = nil
	}
	u.manager = nil
	return nil
}
