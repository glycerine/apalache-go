package spacer

// Source: src/muz/spacer/spacer_unsat_core_learner.h and
// src/muz/spacer/spacer_unsat_core_learner.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/proofs"
)

var ErrUnsatCoreLearnerOwner = errors.New("muz/spacer: invalid or closed unsat-core learner")

type UnsatCorePlugin interface {
	ComputePartialCore(ast.ASTHandle) error
	Finalize() error
	Close() error
}

// UnsatCoreLearner marks proof dependencies and delegates mixed A/B proof
// steps to registered plugins in source order.
type UnsatCoreLearner struct {
	manager  *ast.ASTManager
	plugins  []UnsatCorePlugin
	symbolsB map[ast.ASTHandle]bool
	aMark    map[ast.ASTHandle]bool
	bMark    map[ast.ASTHandle]bool
	hMark    map[ast.ASTHandle]bool
	closed   map[ast.ASTHandle]bool
	core     []ast.ASTRef
	dead     bool
}

func NewUnsatCoreLearner(manager *ast.ASTManager) (*UnsatCoreLearner, error) {
	if manager == nil {
		return nil, ErrUnsatCoreLearnerOwner
	}
	return &UnsatCoreLearner{manager: manager}, nil
}

func (l *UnsatCoreLearner) open() error {
	if l == nil || l.dead || l.manager == nil {
		return ErrUnsatCoreLearnerOwner
	}
	return nil
}

func (l *UnsatCoreLearner) RegisterPlugin(plugin UnsatCorePlugin) error {
	if err := l.open(); err != nil {
		return err
	}
	if plugin == nil {
		return ErrUnsatCoreLearnerOwner
	}
	l.plugins = append(l.plugins, plugin)
	return nil
}

func (l *UnsatCoreLearner) reset() (result error) {
	for i := range l.core {
		result = errors.Join(result, l.manager.DelRef(&l.core[i]))
	}
	l.core = nil
	l.symbolsB = make(map[ast.ASTHandle]bool)
	l.aMark = make(map[ast.ASTHandle]bool)
	l.bMark = make(map[ast.ASTHandle]bool)
	l.hMark = make(map[ast.ASTHandle]bool)
	l.closed = make(map[ast.ASTHandle]bool)
	return result
}

func (l *UnsatCoreLearner) collectSymbols(expression ast.ASTHandle, visited map[ast.ASTHandle]bool) error {
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		kind, err := l.manager.GetKind(current)
		if err != nil {
			return err
		}
		switch kind {
		case ast.ASTApp:
			declaration, err := l.manager.GetDecl(current)
			if err != nil {
				return err
			}
			family, err := l.manager.GetFamilyID(declaration)
			if err != nil {
				return err
			}
			if family == ast.NullFamilyID {
				l.symbolsB[declaration] = true
			}
			count, err := l.manager.GetNumArgs(current)
			if err != nil {
				return err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := l.manager.GetArg(current, i)
				if err != nil {
					return err
				}
				stack = append(stack, argument)
			}
		case ast.ASTQuantifier:
			body, err := l.manager.GetQuantifierBody(current)
			if err != nil {
				return err
			}
			stack = append(stack, body)
			patterns, err := l.manager.GetNumPatterns(current)
			if err != nil {
				return err
			}
			for i := uint32(0); i < patterns; i++ {
				pattern, err := l.manager.GetPattern(current, i)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
			noPatterns, err := l.manager.GetNumNoPatterns(current)
			if err != nil {
				return err
			}
			for i := uint32(0); i < noPatterns; i++ {
				pattern, err := l.manager.GetNoPattern(current, i)
				if err != nil {
					return err
				}
				stack = append(stack, pattern)
			}
		}
	}
	return nil
}

func (l *UnsatCoreLearner) collectSymbolsB(assertedB []ast.ASTHandle) error {
	visited := make(map[ast.ASTHandle]bool)
	for _, expression := range assertedB {
		if err := l.collectSymbols(expression, visited); err != nil {
			return err
		}
	}
	return nil
}

func (l *UnsatCoreLearner) OnlyContainsSymbolsB(expression ast.ASTHandle) (bool, error) {
	if err := l.open(); err != nil {
		return false, err
	}
	reached := make(map[ast.ASTHandle]bool)
	previous := l.symbolsB
	l.symbolsB = reached
	err := l.collectSymbols(expression, make(map[ast.ASTHandle]bool))
	l.symbolsB = previous
	if err != nil {
		return false, err
	}
	for declaration := range reached {
		if !previous[declaration] {
			return false, nil
		}
	}
	arrayFamily, err := l.manager.ArrayFamilyID()
	if err != nil {
		return false, err
	}
	visited := make(map[ast.ASTHandle]bool)
	stack := []ast.ASTHandle{expression}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		kind, err := l.manager.GetKind(current)
		if err != nil {
			return false, err
		}
		if kind == ast.ASTApp {
			family, err := l.manager.GetFamilyID(current)
			if err != nil {
				return false, err
			}
			operation, err := l.manager.GetDeclKind(current)
			if err != nil {
				return false, err
			}
			if family == arrayFamily && operation == ast.ASTArrayOpExt {
				return false, nil
			}
			count, err := l.manager.GetNumArgs(current)
			if err != nil {
				return false, err
			}
			for i := uint32(0); i < count; i++ {
				argument, err := l.manager.GetArg(current, i)
				if err != nil {
					return false, err
				}
				stack = append(stack, argument)
			}
		} else if kind == ast.ASTQuantifier {
			body, err := l.manager.GetQuantifierBody(current)
			if err != nil {
				return false, err
			}
			stack = append(stack, body)
		}
	}
	return true, nil
}

func (l *UnsatCoreLearner) IsAMarked(proof ast.ASTHandle) bool { return l != nil && l.aMark[proof] }
func (l *UnsatCoreLearner) IsBMarked(proof ast.ASTHandle) bool { return l != nil && l.bMark[proof] }
func (l *UnsatCoreLearner) IsHMarked(proof ast.ASTHandle) bool { return l != nil && l.hMark[proof] }
func (l *UnsatCoreLearner) IsClosed(proof ast.ASTHandle) bool  { return l != nil && l.closed[proof] }

func (l *UnsatCoreLearner) SetClosed(proof ast.ASTHandle, value bool) error {
	if err := l.open(); err != nil {
		return err
	}
	l.closed[proof] = value
	return nil
}

func (l *UnsatCoreLearner) IsBPure(proof ast.ASTHandle) (bool, error) {
	if l.IsHMarked(proof) {
		return false, nil
	}
	fact, err := l.manager.GetProofFact(proof)
	if err != nil {
		return false, err
	}
	return l.OnlyContainsSymbolsB(fact)
}

func (l *UnsatCoreLearner) IsBOpen(proof ast.ASTHandle) bool {
	return l.IsBMarked(proof) && !l.IsClosed(proof)
}

func (l *UnsatCoreLearner) AddLemmaToCore(lemma ast.ASTHandle) error {
	if err := l.open(); err != nil {
		return err
	}
	reference, err := l.manager.NewRef(lemma)
	if err != nil {
		return err
	}
	l.core = append(l.core, reference)
	return nil
}

func (l *UnsatCoreLearner) computePartialCore(step ast.ASTHandle) error {
	for _, plugin := range l.plugins {
		if l.IsClosed(step) {
			break
		}
		if err := plugin.ComputePartialCore(step); err != nil {
			return err
		}
	}
	return nil
}

func (l *UnsatCoreLearner) ComputeUnsatCore(root ast.ASTHandle, assertedB []ast.ASTHandle) (_ []ast.ASTRef, resultErr error) {
	if err := l.open(); err != nil {
		return nil, err
	}
	if err := l.reset(); err != nil {
		return nil, err
	}
	rootRef, err := l.manager.NewRef(root)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, l.manager.DelRef(&rootRef)) }()
	if err = proofs.ReduceHypotheses(l.manager, rootRef); err != nil {
		return nil, err
	}
	root, err = l.manager.RefGet(rootRef)
	if err != nil {
		return nil, err
	}
	if err = l.collectSymbolsB(assertedB); err != nil {
		return nil, err
	}
	bSet := make(map[ast.ASTHandle]bool, len(assertedB))
	for _, expression := range assertedB {
		bSet[expression] = true
	}
	iterator, err := proofs.NewProofPostOrder(root, l.manager)
	if err != nil {
		return nil, err
	}
	for {
		hasNext, err := iterator.HasNext()
		if err != nil {
			return nil, err
		}
		if !hasNext {
			break
		}
		current, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		count, err := l.manager.GetProofNumParents(current)
		if err != nil {
			return nil, err
		}
		kind, err := l.manager.GetDeclKind(current)
		if err != nil {
			return nil, err
		}
		if count == 0 {
			switch kind {
			case ast.ASTPrAsserted:
				fact, err := l.manager.GetProofFact(current)
				if err != nil {
					return nil, err
				}
				if bSet[fact] {
					l.bMark[current] = true
				} else {
					l.aMark[current] = true
				}
			case ast.ASTPrHypothesis:
				l.hMark[current] = true
			}
		} else {
			closed := true
			for i := uint32(0); i < count; i++ {
				parent, err := l.manager.GetProofParent(current, i)
				if err != nil {
					return nil, err
				}
				l.aMark[current] = l.aMark[current] || l.aMark[parent]
				l.bMark[current] = l.bMark[current] || l.bMark[parent]
				l.hMark[current] = l.hMark[current] || l.hMark[parent]
				closed = closed && (!l.bMark[parent] || l.closed[parent])
			}
			if kind == ast.ASTPrLemma {
				l.hMark[current] = false
			}
			l.closed[current] = closed
		}
		if l.aMark[current] && l.bMark[current] && !l.closed[current] {
			if err = l.computePartialCore(current); err != nil {
				return nil, err
			}
		}
	}
	for _, plugin := range l.plugins {
		if err = plugin.Finalize(); err != nil {
			return nil, err
		}
	}
	result := make([]ast.ASTRef, 0, len(l.core))
	committed := false
	defer func() {
		if !committed {
			for i := range result {
				resultErr = errors.Join(resultErr, l.manager.DelRef(&result[i]))
			}
		}
	}()
	for i := range l.core {
		handle, err := l.manager.RefGet(l.core[i])
		if err != nil {
			return nil, err
		}
		reference, err := l.manager.NewRef(handle)
		if err != nil {
			return nil, err
		}
		result = append(result, reference)
	}
	committed = true
	return result, nil
}

func (l *UnsatCoreLearner) Close() (result error) {
	if l == nil {
		return ErrUnsatCoreLearnerOwner
	}
	if l.dead {
		return nil
	}
	for i := len(l.plugins) - 1; i >= 0; i-- {
		result = errors.Join(result, l.plugins[i].Close())
	}
	l.plugins = nil
	result = errors.Join(result, l.reset())
	l.manager = nil
	l.dead = true
	return result
}
