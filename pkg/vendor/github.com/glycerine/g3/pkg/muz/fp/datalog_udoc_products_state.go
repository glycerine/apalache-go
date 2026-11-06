package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type udocProductContext struct {
	program       *udocProductProgram
	manager       *rel.RelationManager
	factory       *rel.UDocRelationFactory
	seeds, result map[ast.ASTHandle]*rel.UDocTableProduct
	borrowedSeeds map[ast.ASTHandle]*rel.UDocTableProduct
	pending       []*udocProductAttempt
}

type udocProductAttempt struct {
	values []*rel.UDocTableProduct
	docs   []*rel.TypedUDocRelation
	tables []rel.FiniteTable
	nested memory.ConstructionFailure
}

func (a *udocProductAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	for len(a.values) != 0 {
		last := len(a.values) - 1
		if a.values[last] != nil {
			if err := a.values[last].Close(); err != nil {
				return err
			}
		}
		a.values = a.values[:last]
	}
	for len(a.docs) != 0 {
		last := len(a.docs) - 1
		if err := a.docs[last].Close(); err != nil {
			return err
		}
		a.docs = a.docs[:last]
	}
	for len(a.tables) != 0 {
		last := len(a.tables) - 1
		if err := a.tables[last].Close(); err != nil {
			return err
		}
		a.tables = a.tables[:last]
	}
	return nil
}

func (p *udocProductContext) attempt() *udocProductAttempt {
	a := &udocProductAttempt{}
	p.pending = append(p.pending, a)
	return a
}

func (p *udocProductContext) finish(a *udocProductAttempt, cause error) error {
	if cause != nil {
		errors.As(cause, &a.nested)
	}
	cleanup := a.close()
	if cleanup == nil {
		for i, value := range p.pending {
			if value == a {
				p.pending = append(p.pending[:i], p.pending[i+1:]...)
				break
			}
		}
	}
	return errors.Join(cause, cleanup)
}

func (p *udocProductContext) discardResult() error {
	if p == nil {
		return nil
	}
	for len(p.pending) != 0 {
		last := len(p.pending) - 1
		if err := p.pending[last].close(); err != nil {
			return err
		}
		p.pending = p.pending[:last]
	}
	for d, value := range p.result {
		if err := value.Close(); err != nil {
			return err
		}
		delete(p.result, d)
	}
	p.result = nil
	return nil
}

func (p *udocProductContext) close() error {
	if p == nil {
		return nil
	}
	if err := p.discardResult(); err != nil {
		return err
	}
	for d, value := range p.seeds {
		if err := value.Close(); err != nil {
			return err
		}
		delete(p.seeds, d)
	}
	p.borrowedSeeds, p.manager, p.factory, p.program = nil, nil, nil, nil
	return nil
}

func (p *udocProductContext) clone() *udocProductContext {
	if p == nil {
		return nil
	}
	// newQueryClone immediately calls sync with its new relation manager.
	// Until then these seeds are borrowed from the still-live parent context.
	return &udocProductContext{program: p.program, manager: p.manager, factory: p.factory, borrowedSeeds: p.seeds, seeds: make(map[ast.ASTHandle]*rel.UDocTableProduct)}
}

func (p *udocProductContext) sync(manager *rel.RelationManager, factory *rel.UDocRelationFactory, program *udocProductProgram) (resultErr error) {
	p.manager, p.factory, p.program = manager, factory, program
	if p.seeds == nil {
		p.seeds = make(map[ast.ASTHandle]*rel.UDocTableProduct)
	}
	a := p.attempt()
	defer func() { resultErr = p.finish(a, resultErr) }()
	for d, source := range p.borrowedSeeds {
		if p.seeds[d] != nil {
			continue
		}
		value, err := source.CloneForManager(manager)
		if value != nil {
			p.seeds[d] = value
		}
		if err != nil {
			return err
		}
	}
	p.borrowedSeeds = nil
	return nil
}

func (p *udocProductContext) snapshot() (resultErr error) {
	if p == nil {
		return nil
	}
	if err := p.discardResult(); err != nil {
		return err
	}
	p.result = make(map[ast.ASTHandle]*rel.UDocTableProduct)
	a := p.attempt()
	defer func() { resultErr = p.finish(a, resultErr) }()
	for d, source := range p.seeds {
		value, err := source.Clone()
		if value != nil {
			p.result[d] = value
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *udocProductContext) addFact(d ast.ASTHandle, values []ast.ASTHandle) (resultErr error) {
	a := p.attempt()
	defer func() { resultErr = p.finish(a, resultErr) }()
	seed := p.seeds[d]
	if seed == nil {
		inferred := p.result[d]
		if inferred == nil {
			return ErrUDocDatalogFragment
		}
		var err error
		seed, err = inferred.EmptyLike()
		if seed != nil {
			p.seeds[d] = seed
		}
		if err != nil {
			return err
		}
	}
	return seed.AddFact(values)
}
