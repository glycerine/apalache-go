package qe

// Source: src/qe/qe.cpp:52-305, conjunctions.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/backtrack"
)

// ConjunctionPartitioner classifies and partitions conjunctions. It borrows
// its manager and plugins; SolverContext remains the plugins' sole owner.
type ConjunctionPartitioner struct {
	self    *ConjunctionPartitioner
	manager *ast.ASTManager
	plugins []SolverPlugin
	closed  bool
}

func NewConjunctionPartitioner(manager *ast.ASTManager) (*ConjunctionPartitioner, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	p := &ConjunctionPartitioner{manager: manager}
	p.self = p
	return p, nil
}

func (p *ConjunctionPartitioner) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	return nil
}

// AddPlugin records a borrowed plugin at its source family slot.
func (p *ConjunctionPartitioner) AddPlugin(plugin SolverPlugin) error {
	if err := p.open(); err != nil {
		return err
	}
	if plugin == nil {
		return ast.ErrASTHandle
	}
	family := plugin.FamilyID()
	if family < 0 || family == ast.NullFamilyID {
		return ast.ErrASTDomain
	}
	for len(p.plugins) <= int(family) {
		p.plugins = append(p.plugins, nil)
	}
	if p.plugins[family] != nil {
		return ast.ErrASTDomain
	}
	p.plugins[family] = plugin
	return nil
}

func (p *ConjunctionPartitioner) isUninterpreted(application ast.ASTHandle) (bool, error) {
	family, err := p.manager.GetFamilyID(application)
	if err != nil {
		return false, err
	}
	if family == ast.NullFamilyID || family < 0 || int64(family) >= int64(len(p.plugins)) || p.plugins[family] == nil {
		return true, nil
	}
	return p.plugins[family].IsUninterpreted(application)
}

func closePartitionRefs(manager *ast.ASTManager, refs ...*ast.ASTRef) (err error) {
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i] != nil && *refs[i] != (ast.ASTRef{}) {
			err = errors.Join(err, manager.DelRef(refs[i]))
		}
	}
	return err
}

// rewritePartitionAnd matches the default bool_rewriter::mk_and used by
// conjunctions::get_partition: it removes true and repeated literals,
// returns false for false or a complementary pair, and otherwise preserves
// the original conjunction shape and order.
func rewritePartitionAnd(manager *ast.ASTManager, args []ast.ASTHandle) (ast.ASTHandle, error) {
	filtered := make([]ast.ASTHandle, 0, len(args))
	positive := make(map[ast.ASTHandle]bool, len(args))
	negative := make(map[ast.ASTHandle]bool, len(args))
	changed := false
	for _, arg := range args {
		if manager.IsTrue(arg) {
			changed = true
			continue
		}
		if manager.IsFalse(arg) {
			return manager.MkFalse()
		}
		isNot, atom, err := manager.IsNot(arg)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if isNot {
			if negative[atom] {
				changed = true
				continue
			}
			if positive[atom] {
				return manager.MkFalse()
			}
			negative[atom] = true
		} else {
			if positive[arg] {
				changed = true
				continue
			}
			if negative[arg] {
				return manager.MkFalse()
			}
			positive[arg] = true
		}
		filtered = append(filtered, arg)
	}
	if changed {
		args = filtered
	}
	return manager.MkAndArray(args)
}

// GetPartition returns owning references. Closed conjuncts mention none of the
// supplied variables; mixed conjuncts mention a variable and an uninterpreted
// application; open conjuncts include both mixed and purely interpreted ones.
func (p *ConjunctionPartitioner) GetPartition(formula ast.ASTHandle, variables []ast.ASTHandle) (closed, mixed, open ast.ASTRef, result error) {
	if result = p.open(); result != nil {
		return closed, mixed, open, result
	}
	for _, variable := range variables {
		kind, err := p.manager.GetKind(variable)
		if err != nil || kind != ast.ASTApp {
			return closed, mixed, open, errors.Join(ast.ErrASTDomain, err)
		}
	}
	conjuncts, err := p.manager.FlattenAnd([]ast.ASTHandle{formula})
	if err != nil {
		return closed, mixed, open, err
	}
	visited := make(map[ast.ASTHandle]bool)
	containsVariable := make(map[ast.ASTHandle]bool)
	containsUninterpreted := make(map[ast.ASTHandle]bool)
	todo := append([]ast.ASTHandle(nil), conjuncts...)
	isVariable := func(expression ast.ASTHandle) bool {
		for _, variable := range variables {
			if expression == variable {
				return true
			}
		}
		return false
	}
	for len(todo) > 0 {
		expression := todo[len(todo)-1]
		if visited[expression] {
			todo = todo[:len(todo)-1]
			continue
		}
		if isVariable(expression) {
			containsVariable[expression] = true
			visited[expression] = true
			todo = todo[:len(todo)-1]
			continue
		}
		kind, err := p.manager.GetKind(expression)
		if err != nil {
			return closed, mixed, open, err
		}
		if kind != ast.ASTApp {
			visited[expression] = true
			todo = todo[:len(todo)-1]
			continue
		}
		uninterpreted, err := p.isUninterpreted(expression)
		if err != nil {
			return closed, mixed, open, err
		}
		if uninterpreted {
			containsUninterpreted[expression] = true
		}
		count, err := p.manager.GetNumArgs(expression)
		if err != nil {
			return closed, mixed, open, err
		}
		allVisited := true
		for i := uint32(0); i < count; i++ {
			argument, cause := p.manager.GetArg(expression, i)
			if cause != nil {
				return closed, mixed, open, cause
			}
			if !visited[argument] {
				allVisited = false
				todo = append(todo, argument)
				continue
			}
			if containsVariable[argument] {
				containsVariable[expression] = true
			}
			if containsUninterpreted[argument] {
				containsUninterpreted[expression] = true
			}
		}
		if allVisited {
			todo = todo[:len(todo)-1]
			visited[expression] = true
		}
	}
	var closedHandles, mixedHandles, openHandles []ast.ASTHandle
	for _, expression := range conjuncts {
		cv, cu := containsVariable[expression], containsUninterpreted[expression]
		switch {
		case cv && cu:
			mixedHandles = append(mixedHandles, expression)
			openHandles = append(openHandles, expression)
		case cv:
			openHandles = append(openHandles, expression)
		default:
			closedHandles = append(closedHandles, expression)
		}
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, closePartitionRefs(p.manager, &closed, &mixed, &open))
		}
	}()
	h, err := rewritePartitionAnd(p.manager, closedHandles)
	if err != nil {
		return closed, mixed, open, err
	}
	closed, err = p.manager.NewRef(h)
	if err != nil {
		return closed, mixed, open, err
	}
	h, err = rewritePartitionAnd(p.manager, mixedHandles)
	if err != nil {
		return closed, mixed, open, err
	}
	mixed, err = p.manager.NewRef(h)
	if err != nil {
		return closed, mixed, open, err
	}
	h, err = rewritePartitionAnd(p.manager, openHandles)
	if err != nil {
		return closed, mixed, open, err
	}
	open, err = p.manager.NewRef(h)
	return closed, mixed, open, err
}

// PartitionVariables groups variables whose occurrence sets intersect. A
// variable occurring in a strict majority of conjuncts is placed in the
// source's shared bucket zero and excluded from union-find.
func (p *ConjunctionPartitioner) PartitionVariables(variables []*ast.ContainsApp, conjuncts []ast.ASTHandle) (partition [][]uint32, partitioned bool, err error) {
	if err = p.open(); err != nil {
		return nil, false, err
	}
	containsIndex := make([]uint32, 0, len(conjuncts))
	nonShared := make([]uint32, 0, len(conjuncts))
	nonSharedVariables := make([]uint32, 0, len(variables))
	for range conjuncts {
		nonShared = append(nonShared, math.MaxUint32)
	}
	union := backtrack.NewDefaultUnionFind()
	for variableIndex, contains := range variables {
		if contains == nil {
			return nil, false, ast.ErrASTHandle
		}
		containsIndex = containsIndex[:0]
		for conjunctIndex, expression := range conjuncts {
			yes, cause := contains.Check(expression)
			if cause != nil {
				return nil, false, cause
			}
			if yes {
				containsIndex = append(containsIndex, uint32(conjunctIndex))
			}
		}
		if 2*len(containsIndex) > len(conjuncts) {
			if len(partition) == 0 {
				partition = append(partition, nil)
			}
			partition[len(partition)-1] = append(partition[len(partition)-1], uint32(variableIndex))
			continue
		}
		variable := union.MkVar()
		if variable != uint32(len(nonSharedVariables)) {
			return nil, false, ast.ErrASTDomain
		}
		nonSharedVariables = append(nonSharedVariables, uint32(variableIndex))
		for _, conjunctIndex := range containsIndex {
			previous := nonShared[conjunctIndex]
			if previous != math.MaxUint32 {
				union.Merge(variable, previous)
			} else {
				nonShared[conjunctIndex] = variable
			}
		}
	}
	if len(nonSharedVariables) == 0 {
		return partition, false, nil
	}
	root := union.Find(0)
	for i := uint32(1); i < uint32(len(nonSharedVariables)); i++ {
		if union.Find(i) != root {
			partitioned = true
			break
		}
	}
	if !partitioned {
		return partition, false, nil
	}
	roots := make([]uint32, 0, len(nonSharedVariables)+1)
	if len(partition) != 0 {
		roots = append(roots, math.MaxUint32)
	}
	for i, original := range nonSharedVariables {
		representative := nonSharedVariables[union.Find(uint32(i))]
		found := false
		for bucket, existing := range roots {
			if existing == representative {
				partition[bucket] = append(partition[bucket], original)
				found = true
				break
			}
		}
		if !found {
			roots = append(roots, representative)
			partition = append(partition, []uint32{original})
		}
	}
	if len(partition) == 1 {
		return partition, false, ast.ErrASTDomain
	}
	return partition, true, nil
}

func (p *ConjunctionPartitioner) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	p.plugins = nil
	p.closed = true
	return nil
}
