// Package ddnf implements the disjoint-disjunctive-normal-form engine.
package ddnf

// Source: src/muz/ddnf/ddnf.h ddnf_core; src/muz/ddnf/ddnf.cpp ddnf_node,
// ddnf_mgr, and ddnf_core.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/muz/rel"
)

var ErrCore = errors.New("muz/ddnf: invalid core operation")

type Statistics struct {
	Insertions  uint32
	Comparisons uint32
	Nodes       uint32
}

type Node struct {
	manager     *Core
	vector      *rel.TBV
	children    []*Node
	id          uint32
	descendants map[uint32]struct{}
}

func (n *Node) ID() uint32 { return n.id }

func (n *Node) Vector() *rel.TBV { return n.vector }

func (n *Node) NumChildren() uint32 { return uint32(len(n.children)) }

func (n *Node) Child(index uint32) (*Node, error) {
	if n == nil || index >= uint32(len(n.children)) {
		return nil, arena.ErrBounds
	}
	return n.children[index], nil
}

func (n *Node) ContainsChild(child *Node) bool {
	if n == nil {
		return false
	}
	for _, candidate := range n.children {
		if candidate == child {
			return true
		}
	}
	return false
}

func (n *Node) addChild(child *Node) {
	n.children = append(n.children, child)
}

func (n *Node) removeChild(child *Node) error {
	for i, candidate := range n.children {
		if candidate == child {
			copy(n.children[i:], n.children[i+1:])
			n.children = n.children[:len(n.children)-1]
			return nil
		}
	}
	return ErrCore
}

type Core struct {
	vectors      *rel.TBVManager
	root         *Node
	nodes        []*Node
	byVector     map[string]*Node
	marked       []bool
	stats        Statistics
	internalized bool
	closed       bool
}

func NewCore(width uint32, config arena.VectorConfig) (*Core, error) {
	vectors, err := rel.NewTBVManager(width, config)
	if err != nil {
		return nil, err
	}
	rootVector, err := vectors.AllocateX()
	if err != nil {
		vectors.Close()
		return nil, err
	}
	root := &Node{vector: rootVector, id: 0}
	core := &Core{vectors: vectors, root: root, nodes: []*Node{root}, byVector: make(map[string]*Node)}
	root.manager = core
	key, err := core.key(rootVector)
	if err != nil {
		core.Close()
		return nil, err
	}
	core.byVector[key] = root
	return core, nil
}

func (c *Core) Manager() *rel.TBVManager {
	if c == nil || c.closed {
		return nil
	}
	return c.vectors
}

func (c *Core) Close() {
	if c == nil || c.closed {
		return
	}
	for _, node := range c.nodes {
		node.children = nil
		node.descendants = nil
		node.manager = nil
	}
	c.nodes = nil
	c.byVector = nil
	c.marked = nil
	c.vectors.Close()
	c.closed = true
}

func (c *Core) key(vector *rel.TBV) (string, error) {
	if c == nil || c.closed || c.vectors == nil {
		return "", arena.ErrClosed
	}
	return c.vectors.String(vector)
}

func (c *Core) find(vector *rel.TBV) (*Node, error) {
	key, err := c.key(vector)
	if err != nil {
		return nil, err
	}
	node := c.byVector[key]
	if node == nil {
		return nil, ErrCore
	}
	return node, nil
}

func (c *Core) Contains(vector *rel.TBV) (bool, error) {
	key, err := c.key(vector)
	if err != nil {
		return false, err
	}
	_, found := c.byVector[key]
	return found, nil
}

func (c *Core) Size() uint32 {
	if c == nil || c.closed {
		return 0
	}
	return uint32(len(c.nodes))
}

func (c *Core) Statistics() Statistics {
	if c == nil {
		return Statistics{}
	}
	result := c.stats
	result.Nodes = uint32(len(c.nodes))
	return result
}

func (c *Core) Insert(vector *rel.TBV) (*Node, error) {
	if c == nil || c.closed {
		return nil, arena.ErrClosed
	}
	if c.internalized {
		return nil, ErrCore
	}
	if _, err := c.key(vector); err != nil {
		return nil, err
	}
	newVectors := []*rel.TBV{vector}
	for i := 0; i < len(newVectors); i++ {
		candidateVector := newVectors[i]
		candidate, err := c.find(candidateVector)
		if errors.Is(err, ErrCore) {
			candidate = &Node{manager: c, vector: candidateVector, id: uint32(len(c.nodes))}
			key, keyErr := c.key(candidateVector)
			if keyErr != nil {
				return nil, keyErr
			}
			c.nodes = append(c.nodes, candidate)
			c.byVector[key] = candidate
		} else if err != nil {
			return nil, err
		}
		if err = c.insertUnder(c.root, candidate, &newVectors); err != nil {
			return nil, err
		}
	}
	return c.find(vector)
}

func (c *Core) insertUnder(root, candidate *Node, intersections *[]*rel.TBV) error {
	contains, err := c.vectors.Contains(root.vector, candidate.vector)
	if err != nil {
		return err
	}
	if !contains {
		return ErrCore
	}
	equal, err := c.vectors.Equals(root.vector, candidate.vector)
	if err != nil || equal {
		return err
	}
	c.stats.Insertions++
	inserted := false
	for _, child := range root.children {
		c.stats.Comparisons++
		contains, err = c.vectors.Contains(child.vector, candidate.vector)
		if err != nil {
			return err
		}
		if contains {
			inserted = true
			if err = c.insertUnder(child, candidate, intersections); err != nil {
				return err
			}
		}
	}
	if inserted {
		return nil
	}

	subsetChildren := make([]*Node, 0, len(root.children))
	intersection, err := c.vectors.Allocate()
	if err != nil {
		return err
	}
	intersectionLive := true
	deallocate := func() {
		if intersectionLive {
			_ = c.vectors.Deallocate(intersection)
			intersectionLive = false
		}
	}
	defer deallocate()
	for _, child := range root.children {
		contains, err = c.vectors.Contains(candidate.vector, child.vector)
		if err != nil {
			return err
		}
		if contains {
			subsetChildren = append(subsetChildren, child)
			c.stats.Comparisons++
			continue
		}
		wellFormed, intersectErr := c.vectors.Intersect(child.vector, candidate.vector, intersection)
		c.stats.Comparisons += 2
		if intersectErr != nil {
			return intersectErr
		}
		if wellFormed {
			*intersections = append(*intersections, intersection)
			intersectionLive = false
			intersection, err = c.vectors.Allocate()
			if err != nil {
				return err
			}
			intersectionLive = true
		}
	}
	deallocate()
	for _, child := range subsetChildren {
		if err = root.removeChild(child); err != nil {
			return err
		}
		candidate.addChild(child)
	}
	root.addChild(candidate)
	return nil
}

func (c *Core) ResetAccumulate() {
	if c == nil || c.closed {
		return
	}
	c.marked = make([]bool, len(c.nodes))
}

func (c *Core) Accumulate(vector *rel.TBV, labels []uint32) ([]uint32, error) {
	if c == nil || c.closed {
		return labels, arena.ErrClosed
	}
	if len(c.marked) != len(c.nodes) {
		return labels, ErrCore
	}
	node, err := c.find(vector)
	if err != nil {
		return labels, err
	}
	todo := []*Node{node}
	for len(todo) != 0 {
		node = todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if c.marked[node.id] {
			continue
		}
		labels = append(labels, node.id)
		c.marked[node.id] = true
		todo = append(todo, node.children...)
	}
	return labels, nil
}

func (c *Core) WellFormed() (bool, error) {
	if c == nil || c.closed {
		return false, arena.ErrClosed
	}
	c.ResetAccumulate()
	todo := []*Node{c.root}
	for len(todo) != 0 {
		node := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if c.marked[node.id] {
			continue
		}
		c.marked[node.id] = true
		for _, child := range node.children {
			contains, err := c.vectors.Contains(node.vector, child.vector)
			if err != nil || !contains {
				return false, err
			}
			todo = append(todo, child)
		}
	}
	return true, nil
}

func (c *Core) internalize() error {
	if c.internalized {
		return nil
	}
	done := make([]bool, len(c.nodes))
	todo := []*Node{c.root}
	for len(todo) != 0 {
		node := todo[len(todo)-1]
		if done[node.id] {
			todo = todo[:len(todo)-1]
			continue
		}
		allDone := true
		for _, child := range node.children {
			if !done[child.id] {
				allDone = false
				todo = append(todo, child)
			}
		}
		if !allDone {
			continue
		}
		node.descendants = make(map[uint32]struct{})
		node.descendants[node.id] = struct{}{}
		for _, child := range node.children {
			for id := range child.descendants {
				node.descendants[id] = struct{}{}
			}
		}
		done[node.id] = true
		todo = todo[:len(todo)-1]
	}
	c.internalized = true
	return nil
}

// descendants retains ddnf_mgr::lookup for the later AST engine slice. The
// source lookup freezes insertion; results are returned in stable node-ID order.
func (c *Core) descendants(vector *rel.TBV) ([]*Node, error) {
	if err := c.internalize(); err != nil {
		return nil, err
	}
	node, err := c.find(vector)
	if err != nil {
		return nil, err
	}
	result := make([]*Node, 0, len(node.descendants))
	for _, candidate := range c.nodes {
		if _, found := node.descendants[candidate.id]; found {
			result = append(result, candidate)
		}
	}
	return result, nil
}

func (c *Core) Display() (string, error) {
	if c == nil || c.closed {
		return "", arena.ErrClosed
	}
	var output strings.Builder
	for _, node := range c.nodes {
		text, err := c.vectors.String(node.vector)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&output, "node[%d: %s", node.id, text)
		for _, child := range node.children {
			fmt.Fprintf(&output, " %d", child.id)
		}
		output.WriteString("]\n")
	}
	return output.String(), nil
}

func (c *Core) DisplayStatistics() string {
	stats := c.Statistics()
	return fmt.Sprintf("Number of insertions:  %d\nNumber of comparisons: %d\nNumber of nodes:       %d\n", stats.Insertions, stats.Comparisons, stats.Nodes)
}
