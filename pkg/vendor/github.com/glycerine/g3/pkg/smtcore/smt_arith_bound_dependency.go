package smtcore

// Source: live scoped_dependency_manager<void*> instantiation from
// src/util/dependency.h, consumed by src/smt/old_interval.* and
// src/smt/theory_arith_nl.h.

// smtArithmeticBound is the concrete leaf type used by nonlinear arithmetic
// explanations. Its bound payload is filled by the arithmetic-theory port.
type smtArithmeticBound struct {
	variable          SMTTheoryVar
	value             smtArithmeticValue
	kind              smtArithmeticBoundKind
	atom              bool
	replayExplanation *smtArithmeticBoundExplanation
}

type smtArithmeticBoundDependency struct {
	leaf        *smtArithmeticBound
	left, right *smtArithmeticBoundDependency
}

// smtArithmeticBoundDependencyManager owns dependency nodes by search scope.
// Bound pointers are borrowed, matching the source void-pointer specialization.
type smtArithmeticBoundDependencyManager struct {
	nodes  []*smtArithmeticBoundDependency
	scopes []int
}

func (m *smtArithmeticBoundDependencyManager) Reset() {
	m.nodes = m.nodes[:0]
	m.scopes = m.scopes[:0]
}

func (m *smtArithmeticBoundDependencyManager) PushScope() {
	m.scopes = append(m.scopes, len(m.nodes))
}

func (m *smtArithmeticBoundDependencyManager) PopScopes(count uint32) bool {
	if count == 0 || count > uint32(len(m.scopes)) {
		return false
	}
	limit := m.scopes[len(m.scopes)-int(count)]
	for index := limit; index < len(m.nodes); index++ {
		m.nodes[index] = nil
	}
	m.nodes = m.nodes[:limit]
	m.scopes = m.scopes[:len(m.scopes)-int(count)]
	return true
}

func (m *smtArithmeticBoundDependencyManager) Leaf(value *smtArithmeticBound) *smtArithmeticBoundDependency {
	if value == nil {
		return nil
	}
	result := &smtArithmeticBoundDependency{leaf: value}
	m.nodes = append(m.nodes, result)
	return result
}

func (m *smtArithmeticBoundDependencyManager) Join(left, right *smtArithmeticBoundDependency) *smtArithmeticBoundDependency {
	if left == nil {
		return right
	}
	if right == nil || left == right {
		return left
	}
	result := &smtArithmeticBoundDependency{left: left, right: right}
	m.nodes = append(m.nodes, result)
	return result
}

func (m *smtArithmeticBoundDependencyManager) Contains(root *smtArithmeticBoundDependency, value *smtArithmeticBound) bool {
	if root == nil || value == nil {
		return false
	}
	seen := make(map[*smtArithmeticBoundDependency]bool)
	stack := []*smtArithmeticBoundDependency{root}
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if node == nil || seen[node] {
			continue
		}
		seen[node] = true
		if node.leaf != nil {
			if node.leaf == value {
				return true
			}
			continue
		}
		stack = append(stack, node.left, node.right)
	}
	return false
}

func (m *smtArithmeticBoundDependencyManager) Linearize(root *smtArithmeticBoundDependency, values []*smtArithmeticBound) []*smtArithmeticBound {
	if root == nil {
		return values
	}
	seen := make(map[*smtArithmeticBoundDependency]bool)
	stack := []*smtArithmeticBoundDependency{root}
	for head := 0; head < len(stack); head++ {
		node := stack[head]
		if node == nil || seen[node] {
			continue
		}
		seen[node] = true
		if node.leaf != nil {
			values = append(values, node.leaf)
			continue
		}
		stack = append(stack, node.left, node.right)
	}
	return values
}
