package fp

import (
	"errors"
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type sparseIntervalDependency struct {
	head, tail ast.ASTHandle
	negative   bool
}

// sparseIntervalPredicateStrata mirrors the source rule-set stratification:
// both positive and negative dependencies order components, while a negative
// dependency in a recursive component is rejected as unstratified.
func (e *SparseIntervalDatalogEvaluator) sparseIntervalPredicateStrata() (map[ast.ASTHandle]int, []int, error) {
	nodes := make([]ast.ASTHandle, 0, len(e.sorts))
	for declaration := range e.sorts {
		nodes = append(nodes, declaration)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Origin() != nodes[j].Origin() {
			return nodes[i].Origin() < nodes[j].Origin()
		}
		if nodes[i].Slot() != nodes[j].Slot() {
			return nodes[i].Slot() < nodes[j].Slot()
		}
		return nodes[i].Generation() < nodes[j].Generation()
	})
	graph := make(map[ast.ASTHandle][]ast.ASTHandle, len(nodes))
	dependencies := make([]sparseIntervalDependency, 0)
	count, err := e.rules.Len()
	if err != nil {
		return nil, nil, err
	}
	for i := 0; i < count; i++ {
		rule, cause := e.rules.Rule(i)
		if cause != nil {
			return nil, nil, cause
		}
		head, cause := rule.HeadDeclaration()
		positive, pErr := rule.PositiveTailSize()
		uninterpreted, uErr := rule.UninterpretedTailSize()
		cause = errors.Join(cause, pErr, uErr)
		if _, declared := e.sorts[head]; !declared {
			cause = errors.Join(cause, fmt.Errorf("%w: undeclared rule head", ErrSparseIntervalDatalogFragment))
		}
		seen := make(map[ast.ASTHandle]bool)
		for j := 0; cause == nil && j < uninterpreted; j++ {
			tail, tailErr := rule.TailDeclaration(j)
			cause = errors.Join(cause, tailErr)
			if _, declared := e.sorts[tail]; !declared {
				cause = errors.Join(cause, fmt.Errorf("%w: undeclared rule tail", ErrSparseIntervalDatalogFragment))
				continue
			}
			dependencies = append(dependencies, sparseIntervalDependency{head: head, tail: tail, negative: j >= positive})
			if !seen[tail] {
				seen[tail] = true
				graph[head] = append(graph[head], tail)
			}
		}
		cause = errors.Join(cause, rule.Close())
		if cause != nil {
			return nil, nil, cause
		}
	}
	for head := range graph {
		sort.Slice(graph[head], func(i, j int) bool { return graph[head][i].Slot() < graph[head][j].Slot() })
	}
	indices := make(map[ast.ASTHandle]int, len(nodes))
	low := make(map[ast.ASTHandle]int, len(nodes))
	onStack := make(map[ast.ASTHandle]bool, len(nodes))
	stack := make([]ast.ASTHandle, 0, len(nodes))
	component := make(map[ast.ASTHandle]int, len(nodes))
	nextIndex, componentCount := 0, 0
	var connect func(ast.ASTHandle)
	connect = func(node ast.ASTHandle) {
		nextIndex++
		indices[node], low[node] = nextIndex, nextIndex
		stack = append(stack, node)
		onStack[node] = true
		for _, dependency := range graph[node] {
			if indices[dependency] == 0 {
				connect(dependency)
				if low[dependency] < low[node] {
					low[node] = low[dependency]
				}
			} else if onStack[dependency] && indices[dependency] < low[node] {
				low[node] = indices[dependency]
			}
		}
		if low[node] != indices[node] {
			return
		}
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			component[last] = componentCount
			if last == node {
				break
			}
		}
		componentCount++
	}
	for _, node := range nodes {
		if indices[node] == 0 {
			connect(node)
		}
	}
	componentDependencies := make([]map[int]bool, componentCount)
	for _, dependency := range dependencies {
		headComponent, tailComponent := component[dependency.head], component[dependency.tail]
		if headComponent == tailComponent {
			if dependency.negative {
				return nil, nil, fmt.Errorf("%w: unstratified negation", ErrSparseIntervalDatalogFragment)
			}
			continue
		}
		if componentDependencies[headComponent] == nil {
			componentDependencies[headComponent] = make(map[int]bool)
		}
		componentDependencies[headComponent][tailComponent] = true
	}
	visited := make([]bool, componentCount)
	order := make([]int, 0, componentCount)
	var visit func(int)
	visit = func(current int) {
		if visited[current] {
			return
		}
		visited[current] = true
		deps := make([]int, 0, len(componentDependencies[current]))
		for dependency := range componentDependencies[current] {
			deps = append(deps, dependency)
		}
		sort.Ints(deps)
		for _, dependency := range deps {
			visit(dependency)
		}
		order = append(order, current)
	}
	for current := 0; current < componentCount; current++ {
		visit(current)
	}
	return component, order, nil
}
