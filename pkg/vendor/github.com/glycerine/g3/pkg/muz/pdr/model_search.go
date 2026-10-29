package pdr

// Source: model_node and the retained frontier/cache/backtracking operations
// of model_search in src/muz/pdr/pdr_context.h and pdr_context.cpp.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

var ErrModelSearchOwner = errors.New("muz/pdr: invalid or closed model search")

type ModelNode struct {
	parent      *ModelNode
	next        *ModelNode
	previous    *ModelNode
	transformer *PredicateTransformer
	state       ast.ASTHandle
	stateRef    ast.ASTRef
	model       *model.ModelCore
	rule        *base.Rule
	selected    *normalizedRule
	children    []*ModelNode
	level       uint32
	original    uint32
	depth       uint32
	closed      bool
	released    bool
}

func NewModelNode(parent *ModelNode, state ast.ASTHandle, transformer *PredicateTransformer, level uint32) (*ModelNode, error) {
	if transformer == nil {
		return nil, ErrModelSearchOwner
	}
	if err := transformer.open(); err != nil {
		return nil, err
	}
	if parent != nil {
		if parent.released || parent.transformer == nil || parent.level == 0 || parent.level != level+1 || parent.transformer.owner != transformer.owner {
			return nil, ast.ErrASTDomain
		}
	}
	reference, err := transformer.manager.NewRef(state)
	if err != nil {
		return nil, err
	}
	node := &ModelNode{parent: parent, transformer: transformer, state: state, stateRef: reference, level: level, original: level}
	if parent != nil {
		node.depth = parent.depth + 1
		parent.children = append(parent.children, node)
		if parent.closed {
			if err = parent.SetOpen(); err != nil {
				parent.children = parent.children[:len(parent.children)-1]
				return nil, errors.Join(err, transformer.manager.DelRef(&reference))
			}
		}
	}
	return node, nil
}

func (n *ModelNode) open() error {
	if n == nil || n.released || n.transformer == nil {
		return ErrModelSearchOwner
	}
	return n.transformer.open()
}

func (n *ModelNode) Parent() *ModelNode { return n.parent }

func (n *ModelNode) Transformer() (*PredicateTransformer, error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	return n.transformer, nil
}

func (n *ModelNode) State() (ast.ASTHandle, error) {
	if err := n.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return n.state, nil
}

func (n *ModelNode) Level() uint32         { return n.level }
func (n *ModelNode) OriginalLevel() uint32 { return n.original }
func (n *ModelNode) Depth() uint32         { return n.depth }
func (n *ModelNode) IsClosed() bool        { return n != nil && n.closed }
func (n *ModelNode) IsOpen() bool          { return n != nil && !n.closed }
func (n *ModelNode) IsGoal() bool          { return n != nil && n.next != nil }

func (n *ModelNode) Children() ([]*ModelNode, error) {
	if err := n.open(); err != nil {
		return nil, err
	}
	return append([]*ModelNode(nil), n.children...), nil
}

func (n *ModelNode) IncreaseLevel() error {
	if err := n.open(); err != nil {
		return err
	}
	if n.level == InfinityLevel {
		return ast.ErrASTDomain
	}
	n.level++
	return nil
}

func (n *ModelNode) SetModel(value *model.ModelCore) (result error) {
	if err := n.open(); err != nil {
		return err
	}
	if value == n.model {
		return nil
	}
	if value != nil {
		if err := value.IncRef(); err != nil {
			return err
		}
	}
	prior := n.model
	n.model = value
	if prior != nil {
		result = prior.DecRef()
	}
	return result
}

func (n *ModelNode) Model() (*model.ModelCore, bool, error) {
	if err := n.open(); err != nil {
		return nil, false, err
	}
	return n.model, n.model != nil, nil
}

func (n *ModelNode) SetRule(rule *base.Rule) (result error) {
	if err := n.open(); err != nil {
		return err
	}
	var owned *base.Rule
	var err error
	if rule != nil {
		owned, err = rule.Retain()
		if err != nil {
			return err
		}
	}
	prior := n.rule
	n.rule = owned
	if prior != nil {
		result = prior.Close()
	}
	return result
}

func (n *ModelNode) Rule() (*base.Rule, bool, error) {
	if err := n.open(); err != nil {
		return nil, false, err
	}
	if n.rule == nil {
		return nil, false, nil
	}
	owned, err := n.rule.Retain()
	return owned, err == nil, err
}

func (n *ModelNode) IsOneClosed() bool {
	if n == nil || n.released || n.closed {
		return n != nil && n.closed
	}
	if len(n.children) == 0 {
		return false
	}
	for _, child := range n.children {
		if child.IsOpen() {
			return false
		}
	}
	return true
}

func (n *ModelNode) SetClosed() error {
	if err := n.open(); err != nil {
		return err
	}
	if err := n.transformer.CloseState(n.state); err != nil {
		return err
	}
	n.closed = true
	return nil
}

func (n *ModelNode) SetPreClosed() error {
	if err := n.open(); err != nil {
		return err
	}
	n.closed = true
	return nil
}

func (n *ModelNode) SetOpen() error {
	if err := n.open(); err != nil {
		return err
	}
	if !n.closed {
		return ast.ErrASTDomain
	}
	for current := n; current != nil && current.closed; current = current.parent {
		current.closed = false
	}
	return nil
}

func (n *ModelNode) CheckPreClosed() error {
	if err := n.open(); err != nil {
		return err
	}
	for _, child := range n.children {
		if child.IsOpen() {
			return nil
		}
	}
	n.closed = true
	for current := n.parent; current != nil && current.IsOneClosed(); current = current.parent {
		current.closed = true
	}
	return nil
}

func (n *ModelNode) Index() (uint32, error) {
	if err := n.open(); err != nil {
		return 0, err
	}
	if n.parent == nil {
		return 0, nil
	}
	for index, child := range n.parent.children {
		if child == n {
			return uint32(index), nil
		}
	}
	return 0, ast.ErrASTDomain
}

func (n *ModelNode) enqueue(item *ModelNode) error {
	if item == nil || item.next != nil || item.previous != nil {
		return ast.ErrASTDomain
	}
	if n == item {
		n.next, n.previous = n, n
		return nil
	}
	if n.next == nil || n.previous == nil {
		return ast.ErrASTDomain
	}
	item.next = n.next
	n.next.previous = item
	n.next = item
	item.previous = n
	return nil
}

func (n *ModelNode) dequeue(root **ModelNode) error {
	if n.next == nil && n.previous == nil {
		return nil
	}
	if n.next == nil || n.previous == nil || len(n.children) != 0 {
		return ast.ErrASTDomain
	}
	if n == n.next {
		if n.previous != n {
			return ast.ErrASTDomain
		}
		if *root == n {
			*root = nil
		}
	} else {
		n.next.previous = n.previous
		n.previous.next = n.next
		if *root == n {
			*root = n.next
		}
	}
	n.next, n.previous = nil, nil
	return nil
}

func (n *ModelNode) release() (result error) {
	if n == nil || n.released {
		return nil
	}
	if len(n.children) != 0 || n.next != nil || n.previous != nil {
		return ast.ErrASTDomain
	}
	if n.model != nil {
		result = errors.Join(result, n.model.DecRef())
		n.model = nil
	}
	if n.rule != nil {
		result = errors.Join(result, n.rule.Close())
		n.rule = nil
	}
	result = errors.Join(result, n.transformer.manager.DelRef(&n.stateRef))
	if result == nil {
		n.transformer = nil
		n.parent = nil
		n.released = true
	}
	return result
}

type ModelSearch struct {
	breadthFirst bool
	root         *ModelNode
	goal         *ModelNode
	cache        []map[modelStateKey][]*ModelNode
	closed       bool
}

// modelStateKey keeps obligations for distinct predicates separate even when
// their projected state is syntactically identical. This matters especially
// for zero-arity nonlinear Horn tails, whose state cube is true for every
// predecessor.
type modelStateKey struct {
	transformer *PredicateTransformer
	state       ast.ASTHandle
}

func NewModelSearch(breadthFirst bool) *ModelSearch {
	return &ModelSearch{breadthFirst: breadthFirst}
}

func (s *ModelSearch) open() error {
	if s == nil || s.closed {
		return ErrModelSearchOwner
	}
	return nil
}

func (s *ModelSearch) levelCache(node *ModelNode) map[modelStateKey][]*ModelNode {
	level := node.original
	for uint32(len(s.cache)) <= level {
		s.cache = append(s.cache, make(map[modelStateKey][]*ModelNode))
	}
	return s.cache[level]
}

func modelState(node *ModelNode) modelStateKey {
	return modelStateKey{transformer: node.transformer, state: node.state}
}

func (s *ModelSearch) enqueueLeaf(node *ModelNode) error {
	if !node.IsOpen() || len(node.children) != 0 {
		return ast.ErrASTDomain
	}
	if s.goal == nil {
		s.goal = node
		return node.enqueue(node)
	}
	if s.breadthFirst {
		return s.goal.enqueue(node)
	}
	return s.goal.next.enqueue(node)
}

func (s *ModelSearch) removeGoal(node *ModelNode) error {
	return node.dequeue(&s.goal)
}

func (s *ModelSearch) removeNode(node *ModelNode, backtrack bool) error {
	cache := s.levelCache(node)
	key := modelState(node)
	nodes := cache[key]
	position := -1
	for index, candidate := range nodes {
		if candidate == node {
			position = index
			break
		}
	}
	if position < 0 {
		return ast.ErrASTDomain
	}
	nodes[position] = nodes[len(nodes)-1]
	nodes = nodes[:len(nodes)-1]
	if err := s.removeGoal(node); err != nil {
		return err
	}
	if len(nodes) != 0 && backtrack && len(nodes[0].children) == 0 && nodes[0].closed {
		if err := nodes[0].SetOpen(); err != nil {
			return err
		}
		if err := s.enqueueLeaf(nodes[0]); err != nil {
			return err
		}
	}
	if len(nodes) != 0 && node.model != nil && backtrack {
		if err := nodes[0].SetModel(node.model); err != nil {
			return err
		}
		nodes[0].selected = node.selected
	}
	if len(nodes) == 0 {
		delete(cache, key)
	} else {
		cache[key] = nodes
	}
	return nil
}

func (s *ModelSearch) eraseChildren(node *ModelNode, backtrack bool) (result error) {
	stack := append([]*ModelNode(nil), node.children...)
	node.children = nil
	result = errors.Join(result, s.removeGoal(node))
	removed := make([]*ModelNode, 0, len(stack))
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		stack = append(stack, current.children...)
		current.children = nil
		result = errors.Join(result, s.removeNode(current, backtrack))
		removed = append(removed, current)
	}
	for index := len(removed) - 1; index >= 0; index-- {
		result = errors.Join(result, removed[index].release())
	}
	return result
}

func (s *ModelSearch) setLeaf(node *ModelNode) error {
	if err := s.eraseChildren(node, true); err != nil {
		return err
	}
	if !node.IsOpen() {
		return ast.ErrASTDomain
	}
	return s.enqueueLeaf(node)
}

func (s *ModelSearch) SetRoot(root *ModelNode) error {
	if err := s.open(); err != nil {
		return err
	}
	if root == nil || root.released || root.parent != nil {
		return ast.ErrASTDomain
	}
	if err := s.Reset(); err != nil {
		return err
	}
	s.root = root
	cache := s.levelCache(root)
	key := modelState(root)
	if len(cache[key]) != 0 {
		return ast.ErrASTDomain
	}
	cache[key] = []*ModelNode{root}
	return s.setLeaf(root)
}

func (s *ModelSearch) Root() (*ModelNode, bool, error) {
	if err := s.open(); err != nil {
		return nil, false, err
	}
	return s.root, s.root != nil, nil
}

func (s *ModelSearch) AddLeaf(node *ModelNode) error {
	if err := s.open(); err != nil {
		return err
	}
	if node == nil || node.released || len(node.children) != 0 {
		return ast.ErrASTDomain
	}
	cache := s.levelCache(node)
	key := modelState(node)
	nodes := cache[key]
	for _, prior := range nodes {
		if prior == node {
			return nil
		}
	}
	nodes = append(nodes, node)
	cache[key] = nodes
	if len(nodes) == 1 {
		return s.setLeaf(node)
	}
	return node.SetPreClosed()
}

func (s *ModelSearch) Next() (*ModelNode, bool, error) {
	if err := s.open(); err != nil {
		return nil, false, err
	}
	if s.goal == nil {
		return nil, false, nil
	}
	result := s.goal
	if err := result.dequeue(&s.goal); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func (s *ModelSearch) GoalCount() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if s.goal == nil {
		return 0, nil
	}
	count := uint32(0)
	for node := s.goal; ; node = node.next {
		count++
		if node.next == s.goal {
			break
		}
	}
	return count, nil
}

func (s *ModelSearch) BacktrackLevel(usesLevel bool, node *ModelNode) error {
	if err := s.open(); err != nil {
		return err
	}
	if s.root == nil || node == nil || node.released {
		return ast.ErrASTDomain
	}
	if usesLevel && s.root.level > node.level {
		if err := node.IncreaseLevel(); err != nil {
			return err
		}
		return s.enqueueLeaf(node)
	}
	if node.parent != nil {
		return s.setLeaf(node.parent)
	}
	return nil
}

// WellFormed validates the retained native frontier and closure invariants.
func (s *ModelSearch) WellFormed() error {
	if err := s.open(); err != nil {
		return err
	}
	if s.root == nil {
		if s.goal != nil {
			return ast.ErrASTDomain
		}
		return nil
	}
	nodes := []*ModelNode{s.root}
	for index := 0; index < len(nodes); index++ {
		node := nodes[index]
		if len(node.children) != 0 {
			nodes = append(nodes, node.children...)
		} else if node.IsOpen() && !node.IsGoal() && node.parent != nil {
			return fmt.Errorf("%w: open leaf is missing from frontier", ErrModelSearchOwner)
		}
	}
	openStates := make(map[uint32]map[modelStateKey]bool)
	if s.goal != nil {
		for node := s.goal; ; node = node.next {
			if !node.IsOpen() || len(node.children) != 0 {
				return fmt.Errorf("%w: invalid frontier node", ErrModelSearchOwner)
			}
			states := openStates[node.level]
			if states == nil {
				states = make(map[modelStateKey]bool)
				openStates[node.level] = states
			}
			key := modelState(node)
			if states[key] {
				return fmt.Errorf("%w: duplicate open state at level", ErrModelSearchOwner)
			}
			states[key] = true
			if node.next == s.goal {
				break
			}
		}
	}
	for _, node := range nodes {
		if len(node.children) == 0 || node.parent == nil {
			continue
		}
		openChild := false
		for _, child := range node.children {
			openChild = openChild || child.IsOpen()
		}
		if node.IsOpen() != openChild {
			return fmt.Errorf("%w: internal closure state is inconsistent", ErrModelSearchOwner)
		}
	}
	return nil
}

func (s *ModelSearch) Reset() (result error) {
	if err := s.open(); err != nil {
		return err
	}
	if s.root != nil {
		result = errors.Join(result, s.eraseChildren(s.root, false))
		result = errors.Join(result, s.removeNode(s.root, false))
		result = errors.Join(result, s.root.release())
		s.root = nil
	}
	s.goal = nil
	s.cache = nil
	return result
}

func (s *ModelSearch) Close() error {
	if s == nil {
		return ErrModelSearchOwner
	}
	if s.closed {
		return nil
	}
	if err := s.Reset(); err != nil {
		return err
	}
	s.closed = true
	return nil
}
