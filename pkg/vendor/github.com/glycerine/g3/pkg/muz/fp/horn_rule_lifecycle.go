package fp

// Source: pdr_dl_interface::check_reset and
// spacer_dl_interface::check_reset transformed-rule retention.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// hornRuleSetSubsumes reports whether every new rule is covered by an old
// rule. Only then may learned frame properties survive. Each caller supplies
// the rule sets from its source lifecycle stage.
func hornRuleSetSubsumes(oldSet, newSet *base.RuleSet) (_ bool, resultErr error) {
	if oldSet == nil || newSet == nil {
		return false, nil
	}
	missing, err := firstUncoveredHornRule(oldSet, newSet)
	return err == nil && missing < 0, err
}

// firstUncoveredHornRule preserves source rule order for check_reset's
// diagnostic, without rescanning or holding an extra rule owner.
func firstUncoveredHornRule(oldSet, newSet *base.RuleSet) (_ int, resultErr error) {
	oldCount, err := oldSet.Len()
	if err != nil {
		return -1, err
	}
	newCount, err := newSet.Len()
	if err != nil {
		return -1, err
	}
	for newIndex := 0; newIndex < newCount; newIndex++ {
		candidate, err := newSet.Rule(newIndex)
		if err != nil {
			return -1, err
		}
		covered := false
		for oldIndex := 0; oldIndex < oldCount && !covered; oldIndex++ {
			oldRule, oldErr := oldSet.Rule(oldIndex)
			if oldErr == nil {
				covered, oldErr = base.BasicRuleSubsumes(oldRule, candidate)
			}
			if oldRule != nil {
				oldErr = errors.Join(oldErr, oldRule.Close())
			}
			if oldErr != nil {
				return -1, errors.Join(oldErr, candidate.Close())
			}
		}
		if err = candidate.Close(); err != nil {
			return -1, err
		}
		if !covered {
			return newIndex, nil
		}
	}
	return -1, nil
}

func snapshotHornRules(set *base.RuleSet) (*base.RuleSet, error) {
	if set == nil {
		return nil, ast.ErrASTDomain
	}
	rules, err := set.RuleManager()
	if err != nil {
		return nil, err
	}
	snapshot, err := base.NewRuleSet(rules)
	if err != nil {
		return nil, err
	}
	if err = snapshot.AddRules(set); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	if err = snapshot.InheritPredicates(set); err != nil {
		return nil, errors.Join(err, snapshot.Close())
	}
	return snapshot, nil
}
