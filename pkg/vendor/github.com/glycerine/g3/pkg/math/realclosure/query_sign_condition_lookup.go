package realclosure

// Source: src/math/realclosure/realclosure.cpp:3399-3438.

import "github.com/glycerine/g3/pkg/arena"

func (s *realValueStore) signConditionAt(value *realValue, index int) (*signCondition, *signDet, error) {
	if index < 0 || value == nil || value.isRational {
		return nil, nil, nil
	}
	if s == nil || value.store != s || value.deleted || value.refs == 0 || value.root == nil {
		return nil, nil, arena.ErrInvalidSlot
	}
	root := value.root
	if _, err := root.registryIndex(); err != nil {
		return nil, nil, err
	}
	if root.signs == nil {
		return nil, nil, nil
	}
	if root.signs.deleted || root.signIndex < 0 || root.signIndex >= len(root.signs.conditions) {
		return nil, nil, arena.ErrInvalidSlot
	}
	condition := root.signs.conditions[root.signIndex]
	for condition != nil && index > 0 {
		condition = condition.previous
		index--
	}
	if condition != nil && condition.deleted {
		return nil, nil, arena.ErrInvalidSlot
	}
	return condition, root.signs, nil
}
