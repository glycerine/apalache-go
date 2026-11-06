package base

// Source: src/muz/base/dl_rule_transformer.cpp and .h. This file ports the
// owned, priority-ordered rule transformation pipeline.

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

var ErrRuleTransformerOwner = errors.New("muz/base: invalid or closed rule transformer")

// RemoveDuplicateRuleTails implements rule_transformer::plugin's shared tail
// cleanup. Positive and negative occurrences are distinct. Erasure is the
// source vector's swap-with-last operation, so survivors are deliberately not
// stable when a duplicate is removed before the end.
func RemoveDuplicateRuleTails(tails []ast.ASTHandle, negated []bool) ([]ast.ASTHandle, []bool, error) {
	if len(tails) != len(negated) {
		return nil, nil, fmt.Errorf("muz/base: negation count does not match tail size")
	}
	seen := [2]map[ast.ASTHandle]struct{}{
		make(map[ast.ASTHandle]struct{}),
		make(map[ast.ASTHandle]struct{}),
	}
	for index := 0; index < len(tails); {
		set := 0
		if negated[index] {
			set = 1
		}
		if _, found := seen[set][tails[index]]; !found {
			seen[set][tails[index]] = struct{}{}
			index++
			continue
		}
		last := len(tails) - 1
		if index != last {
			tails[index], negated[index] = tails[last], negated[last]
		}
		tails = tails[:last]
		negated = negated[:last]
	}
	return tails, negated, nil
}

// RuleTransformPlugin is an owned transformation stage. Transform returns nil
// when it made no change and transfers any non-nil RuleSet owner to its caller.
type RuleTransformPlugin interface {
	Priority() uint32
	CanDestratifyNegation() bool
	Transform(*RuleSet) (*RuleSet, error)
	Cancel()
	Close() error
}

// RuleTransformer owns every registered plugin until Reset or Close.
type RuleTransformer struct {
	plugins        []RuleTransformPlugin
	dirty          bool
	closed         bool
	cleanupPending bool
}

func NewRuleTransformer() *RuleTransformer { return &RuleTransformer{} }

func (t *RuleTransformer) open() error {
	if t == nil || t.closed || t.cleanupPending {
		return ErrRuleTransformerOwner
	}
	return nil
}

// Register transfers plugin ownership to the transformer.
func (t *RuleTransformer) Register(plugin RuleTransformPlugin) error {
	if err := t.open(); err != nil {
		return err
	}
	if plugin == nil {
		return ErrRuleTransformerOwner
	}
	t.plugins = append(t.plugins, plugin)
	t.dirty = true
	return nil
}

func (t *RuleTransformer) order() {
	if !t.dirty {
		return
	}
	sort.SliceStable(t.plugins, func(i, j int) bool {
		return t.plugins[i].Priority() > t.plugins[j].Priority()
	})
	t.dirty = false
}

func cloneFinalizedRules(source *RuleSet) (_ *RuleSet, resultErr error) {
	manager, err := source.RuleManager()
	if err != nil {
		return nil, err
	}
	copySet, err := NewRuleSet(manager)
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep || resultErr != nil {
			resultErr = errors.Join(resultErr, copySet.Close())
		}
	}()
	if err = copySet.AddRules(source); err != nil {
		return nil, err
	}
	valid, err := copySet.Finalize()
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, fmt.Errorf("muz/base: source rules are not stratified")
	}
	keep = true
	return copySet, nil
}

func finalizeTransformResult(result *RuleSet) (bool, error) {
	finalized, err := result.IsFinalized()
	if err != nil {
		return false, err
	}
	if finalized {
		return true, nil
	}
	return result.Finalize()
}

// Apply runs plugins from highest to lowest priority and replaces rules when
// at least one stage returns a changed set.
func (t *RuleTransformer) Apply(rules *RuleSet) (modified bool, resultErr error) {
	if err := t.open(); err != nil {
		return false, err
	}
	if rules == nil {
		return false, ErrRuleSetOwner
	}
	finalized, err := rules.IsFinalized()
	if err != nil {
		return false, err
	}
	if !finalized {
		return false, fmt.Errorf("muz/base: finalize rules before transformation")
	}
	t.order()
	current, err := cloneFinalizedRules(rules)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, current.Close()) }()
	for _, plugin := range t.plugins {
		canceled, cancelErr := rules.manager.manager.Canceled()
		if cancelErr != nil {
			return modified, cancelErr
		}
		if canceled {
			break
		}
		next, e := plugin.Transform(current)
		if e != nil {
			if next != nil && next != current {
				e = errors.Join(e, next.Close())
			}
			return modified, e
		}
		if next == nil {
			continue
		}
		if next == current {
			return modified, fmt.Errorf("muz/base: transform plugin returned its borrowed source")
		}
		valid, e := finalizeTransformResult(next)
		if e != nil {
			return modified, errors.Join(e, next.Close())
		}
		if !valid {
			if plugin.CanDestratifyNegation() {
				if e = next.Close(); e != nil {
					return modified, e
				}
				continue
			}
			return modified, errors.Join(fmt.Errorf("muz/base: transform produced unstratified negation"), next.Close())
		}
		if e = current.Close(); e != nil {
			return modified, errors.Join(e, next.Close())
		}
		current = next
		modified = true
	}
	if !modified {
		return false, nil
	}
	if err = rules.ReplaceRules(current); err != nil {
		return false, err
	}
	valid, err := rules.Finalize()
	if err != nil {
		return false, err
	}
	if !valid {
		return false, fmt.Errorf("muz/base: finalized transform became unstratified during replacement")
	}
	return true, nil
}

func (t *RuleTransformer) Cancel() {
	if t == nil || t.closed || t.cleanupPending {
		return
	}
	for _, plugin := range t.plugins {
		plugin.Cancel()
	}
}

// Reset retires only successfully closed plugins. Ordinary use is blocked
// after a refusal until Reset or Close finishes the remaining cleanup.
func (t *RuleTransformer) Reset() error {
	if t == nil || t.closed {
		return ErrRuleTransformerOwner
	}
	t.cleanupPending = true
	for len(t.plugins) > 0 {
		last := len(t.plugins) - 1
		if err := t.plugins[last].Close(); err != nil {
			return err
		}
		t.plugins[last] = nil
		t.plugins = t.plugins[:last]
	}
	t.plugins = nil
	t.dirty = false
	t.cleanupPending = false
	return nil
}

func (t *RuleTransformer) Close() error {
	if t == nil {
		return ErrRuleTransformerOwner
	}
	if t.closed {
		return nil
	}
	if err := t.Reset(); err != nil {
		return err
	}
	t.closed = true
	return nil
}
