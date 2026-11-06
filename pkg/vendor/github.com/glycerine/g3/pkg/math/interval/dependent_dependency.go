package interval

// Source: src/util/dependency.h:25-197, concrete scoped_dependency_manager<unsigned>
// retained by src/math/interval/dep_intervals.h.

type IntervalDependency struct {
	leaf        bool
	value       uint32
	left, right *IntervalDependency
}

type IntervalDependencyManager struct{}

func (*IntervalDependencyManager) Leaf(value uint32) *IntervalDependency {
	return &IntervalDependency{leaf: true, value: value}
}

func (*IntervalDependencyManager) Join(left, right *IntervalDependency) *IntervalDependency {
	if left == nil {
		return right
	}
	if right == nil || left == right {
		return left
	}
	return &IntervalDependency{left: left, right: right}
}

func (*IntervalDependencyManager) Linearize(root *IntervalDependency) []uint32 {
	if root == nil {
		return nil
	}
	queue, seen := []*IntervalDependency{root}, map[*IntervalDependency]bool{root: true}
	values := make([]uint32, 0)
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		if node.leaf {
			values = append(values, node.value)
			continue
		}
		for _, child := range []*IntervalDependency{node.left, node.right} {
			if child != nil && !seen[child] {
				seen[child] = true
				queue = append(queue, child)
			}
		}
	}
	return values
}

func (m *IntervalDependencyManager) Contains(root *IntervalDependency, value uint32) bool {
	for _, item := range m.Linearize(root) {
		if item == value {
			return true
		}
	}
	return false
}
