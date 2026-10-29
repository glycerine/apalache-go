package base

// Source: src/muz/base/dl_rule_transformer.cpp and .h. This file ports the
// owned, priority-ordered rule transformation pipeline.

import (
	"errors"
	"fmt"
	"sort"
)

var ErrRuleTransformerOwner = errors.New("muz/base: invalid or closed rule transformer")

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
	plugins []RuleTransformPlugin
	dirty   bool
	closed  bool
}

func NewRuleTransformer() *RuleTransformer { return &RuleTransformer{} }

func (t *RuleTransformer) open() error {
	if t == nil || t.closed {
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
	if t == nil || t.closed {
		return
	}
	for _, plugin := range t.plugins {
		plugin.Cancel()
	}
}

func (t *RuleTransformer) Reset() (result error) {
	if err := t.open(); err != nil {
		return err
	}
	for i := len(t.plugins) - 1; i >= 0; i-- {
		result = errors.Join(result, t.plugins[i].Close())
	}
	t.plugins = nil
	t.dirty = false
	return result
}

func (t *RuleTransformer) Close() error {
	if t == nil {
		return ErrRuleTransformerOwner
	}
	if t.closed {
		return nil
	}
	result := t.Reset()
	t.closed = true
	return result
}
