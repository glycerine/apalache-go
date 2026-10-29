package mincut

// Source: src/util/min_cut.h:25-59; src/util/min_cut.cpp:21-230.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var (
	ErrNode     = errors.New("g3: min-cut node is out of range")
	ErrGraph    = errors.New("g3: min-cut graph must be acyclic and every reached positive-capacity node must reach the sink")
	ErrComputed = errors.New("g3: min-cut graph was already computed")
)

type minCutEdge struct {
	node   uint32
	weight uint32
}

// MinCut computes the source algorithm's unit or weighted cut from node 0 to
// node 1. ComputeMinCut consumes residual capacity, so each graph is computed
// at most once.
type MinCut struct {
	edges       [][]minCutEdge
	distance    []uint32
	predecessor []uint32
	trace       *diagnostic.TraceTags
	computed    bool
}

// BindTrace borrows an isolated trace stream. Nil restores the process stream.
func (m *MinCut) BindTrace(tags *diagnostic.TraceTags) error {
	if m == nil {
		return ErrNode
	}
	m.trace = tags
	return nil
}

// New creates the distinguished source node 0 and sink node 1.
func New() *MinCut {
	return &MinCut{edges: make([][]minCutEdge, 2)}
}

// NewNode appends a graph node and returns its index.
func (m *MinCut) NewNode() uint32 {
	node := uint32(len(m.edges))
	m.edges = append(m.edges, nil)
	return node
}

// AddEdge adds a unit-capacity directed edge.
func (m *MinCut) AddEdge(from, to uint32) error {
	return m.AddEdgeCapacity(from, to, 1)
}

// AddEdgeCapacity adds a directed edge with the supplied capacity.
func (m *MinCut) AddEdgeCapacity(from, to, capacity uint32) error {
	if m == nil || uint64(from) >= uint64(len(m.edges)) || uint64(to) >= uint64(len(m.edges)) {
		return ErrNode
	}
	m.edges[from] = append(m.edges[from], minCutEdge{node: to, weight: capacity})
	m.traceAddEdge(from, to)
	return nil
}

// ComputeMinCut appends the target endpoints of residual cut edges to cut.
// The input slice is retained only by the caller.
func (m *MinCut) ComputeMinCut(cut []uint32) ([]uint32, error) {
	if m == nil || len(m.edges) < 2 {
		return cut, ErrNode
	}
	if len(m.edges) == 2 {
		return cut, nil
	}
	if m.computed {
		return cut, ErrComputed
	}
	if !m.validInitialGraph() {
		return cut, ErrGraph
	}
	m.computed = true

	m.distance = make([]uint32, len(m.edges))
	m.predecessor = make([]uint32, len(m.edges))
	m.computeInitialDistances()

	var current uint32
	for m.distance[0] < uint32(len(m.edges)) {
		next := m.admissibleEdge(current)
		if next < uint32(len(m.edges)) {
			m.predecessor[next] = current
			current = next
			if current == 1 {
				m.augmentPath()
				current = 0
			}
		} else {
			m.computeDistance(current)
			if current != 0 {
				current = m.predecessor[current]
			}
		}
	}

	reachable := make([]bool, len(m.edges))
	m.computeReachableNodes(reachable)
	return m.appendCutNodes(reachable, cut), nil
}

func (m *MinCut) validInitialGraph() bool {
	reachable := make([]bool, len(m.edges))
	stack := []uint32{0}
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if reachable[node] {
			continue
		}
		reachable[node] = true
		for _, edge := range m.edges[node] {
			stack = append(stack, edge.node)
		}
	}
	indegree := make([]uint32, len(m.edges))
	var reached uint32
	for node, yes := range reachable {
		if !yes {
			continue
		}
		reached++
		for _, edge := range m.edges[node] {
			indegree[edge.node]++
		}
	}
	queue := make([]uint32, 0, reached)
	for node, yes := range reachable {
		if yes && indegree[node] == 0 {
			queue = append(queue, uint32(node))
		}
	}
	var ordered uint32
	for len(queue) != 0 {
		node := queue[0]
		queue = queue[1:]
		ordered++
		for _, edge := range m.edges[node] {
			indegree[edge.node]--
			if indegree[edge.node] == 0 {
				queue = append(queue, edge.node)
			}
		}
	}
	if ordered != reached {
		return false
	}

	positiveFromSource := make([]bool, len(m.edges))
	stack = append(stack[:0], 0)
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if positiveFromSource[node] {
			continue
		}
		positiveFromSource[node] = true
		for _, edge := range m.edges[node] {
			if edge.weight > 0 {
				stack = append(stack, edge.node)
			}
		}
	}
	reverse := make([][]uint32, len(m.edges))
	for node := range m.edges {
		for _, edge := range m.edges[node] {
			if edge.weight > 0 {
				reverse[edge.node] = append(reverse[edge.node], uint32(node))
			}
		}
	}
	positiveToSink := make([]bool, len(m.edges))
	stack = append(stack[:0], 1)
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		if positiveToSink[node] {
			continue
		}
		positiveToSink[node] = true
		stack = append(stack, reverse[node]...)
	}
	for node, yes := range positiveFromSource {
		if yes && !positiveToSink[node] {
			return false
		}
	}
	return positiveFromSource[1]
}

func (m *MinCut) computeInitialDistances() {
	todo := []uint32{0}
	visited := make([]bool, len(m.edges))
	for len(todo) != 0 {
		current := todo[len(todo)-1]
		if visited[current] {
			todo = todo[:len(todo)-1]
			continue
		}
		hasUnvisitedParent := false
		for _, edge := range m.edges[current] {
			if !visited[edge.node] {
				todo = append(todo, edge.node)
				hasUnvisitedParent = true
			}
		}
		if !hasUnvisitedParent {
			visited[current] = true
			todo = todo[:len(todo)-1]
			m.computeDistance(current)
		}
	}
}

func (m *MinCut) admissibleEdge(node uint32) uint32 {
	for _, edge := range m.edges[node] {
		if edge.weight > 0 && m.distance[node] == m.distance[edge.node]+1 {
			return edge.node
		}
	}
	return uint32(len(m.edges))
}

func (m *MinCut) augmentPath() {
	bottleneck := uint32(math.MaxUint32)
	for node := uint32(1); node != 0; node = m.predecessor[node] {
		previous := m.predecessor[node]
		for _, edge := range m.edges[previous] {
			if edge.node == node && edge.weight < bottleneck {
				bottleneck = edge.weight
			}
		}
	}

	for node := uint32(1); node != 0; node = m.predecessor[node] {
		previous := m.predecessor[node]
		for i := range m.edges[previous] {
			if m.edges[previous][i].node == node {
				m.edges[previous][i].weight -= bottleneck
			}
		}
		reverseExists := false
		for i := range m.edges[node] {
			if m.edges[node][i].node == previous {
				reverseExists = true
				m.edges[node][i].weight += bottleneck
			}
		}
		if !reverseExists {
			// min_cut.cpp adds edge(1, max) here. On a multi-edge path that
			// creates a sink self-loop and can repeat augmentation forever;
			// target the actual predecessor to retain a terminating residual graph.
			m.edges[node] = append(m.edges[node], minCutEdge{node: previous, weight: bottleneck})
		}
	}
}

func (m *MinCut) computeDistance(node uint32) {
	if node == 1 {
		m.distance[1] = 0
		return
	}
	minimum := uint32(math.MaxUint32)
	for _, edge := range m.edges[node] {
		if edge.weight > 0 {
			candidate := m.distance[edge.node] + 1
			if candidate < minimum {
				minimum = candidate
			}
		}
	}
	m.distance[node] = minimum
}

func (m *MinCut) computeReachableNodes(reachable []bool) {
	todo := []uint32{0}
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if reachable[current] {
			continue
		}
		reachable[current] = true
		for _, edge := range m.edges[current] {
			if edge.weight > 0 {
				todo = append(todo, edge.node)
			}
		}
	}
}

func (m *MinCut) appendCutNodes(reachable []bool, cut []uint32) []uint32 {
	todo := []uint32{0}
	visited := make([]bool, len(m.edges))
	for len(todo) != 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		for _, edge := range m.edges[current] {
			if reachable[edge.node] {
				todo = append(todo, edge.node)
			} else {
				cut = append(cut, edge.node)
			}
		}
	}
	return cut
}
