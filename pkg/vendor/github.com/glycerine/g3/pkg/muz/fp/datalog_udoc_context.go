package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var ErrUDocDatalogFragment = errors.New("muz/fp: unsupported doc Datalog fragment")

// The source doc plugin owns one width-indexed manager cache. Expression
// query copies borrow that cache and independently own their relation values.
type datalogUDocContext struct {
	manager            *ast.ASTManager
	relations          *rel.RelationManager // borrowed codec for finite source rows
	factory            *rel.UDocRelationFactory
	borrowed           bool
	seeds              map[ast.ASTHandle]*rel.TypedUDocRelation
	result             map[ast.ASTHandle]*rel.TypedUDocRelation
	order              []ast.ASTHandle
	guardCleanup       []*udocGuardAttempt
	finiteNullary      map[ast.ASTHandle]rel.FiniteTable // borrowed from engine relation storage
	finiteOrder        []ast.ASTHandle
	nullaryResult      map[ast.ASTHandle]bool
	finiteInputs       map[ast.ASTHandle]*udocFiniteInput
	inputOrder         []ast.ASTHandle
	finiteResult       map[ast.ASTHandle]rel.FiniteTable
	finiteUnionCleanup []*udocFiniteUnionAttempt
	products           *udocProductContext
}

func newDatalogUDocContext(manager *ast.ASTManager) (*datalogUDocContext, error) {
	factory, err := rel.NewUDocRelationFactory(manager, arena.VectorConfig{})
	if err != nil {
		return nil, err
	}
	return &datalogUDocContext{manager: manager, factory: factory, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation)}, nil
}

func (c *datalogUDocContext) discardResult() error {
	if err := c.products.discardResult(); err != nil {
		return err
	}
	if err := c.clearFiniteUnionCleanup(); err != nil {
		return err
	}
	if err := c.clearGuardCleanup(); err != nil {
		return err
	}
	for d, table := range c.finiteResult {
		if err := table.Close(); err != nil {
			return err
		}
		delete(c.finiteResult, d)
	}
	c.finiteResult = nil
	for d, r := range c.result {
		if err := r.Close(); err != nil {
			return err
		}
		delete(c.result, d)
	}
	c.result = nil
	c.nullaryResult = nil
	return nil
}

func (c *datalogUDocContext) close() error {
	if c == nil {
		return nil
	}
	if err := c.discardResult(); err != nil {
		return err
	}
	if err := c.products.close(); err != nil {
		return err
	}
	c.products = nil
	for d, r := range c.seeds {
		if err := r.Close(); err != nil {
			return err
		}
		delete(c.seeds, d)
	}
	if !c.borrowed && c.factory != nil {
		c.factory.Close()
	}
	c.factory = nil
	c.finiteNullary = nil
	c.finiteOrder = nil
	c.finiteInputs = nil
	c.inputOrder = nil
	c.relations = nil
	return nil
}

func (c *datalogUDocContext) sync(e *PropositionalEngine) error {
	if c.products != nil {
		return c.syncExistingProducts(e)
	}
	c.relations = e.relations
	// Validate the whole new signature set before creating or publishing owners.
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if !c.factory.FormulaChecked() && udocFiniteForwardingInput(state) {
			supported, err := c.factory.CanHandleSignature(state.sorts)
			if err != nil || !supported || c.seeds[d] != nil {
				return errors.Join(err, ErrUDocDatalogFragment)
			}
			continue
		}
		if udocFiniteNullary(state) {
			if c.seeds[d] != nil {
				return ErrUDocDatalogFragment
			}
			continue
		}
		supported, err := c.factory.CanHandleSignature(state.sorts)
		if err != nil {
			return err
		}
		if !supported {
			return ErrUDocDatalogFragment
		}
	}
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		if !c.factory.FormulaChecked() && udocFiniteForwardingInput(state) {
			if c.finiteInputs == nil {
				c.finiteInputs = make(map[ast.ASTHandle]*udocFiniteInput)
			}
			if c.finiteInputs[d] == nil {
				c.inputOrder = append(c.inputOrder, d)
			}
			c.finiteInputs[d] = &udocFiniteInput{table: state.table, sorts: append([]ast.ASTHandle(nil), state.sorts...), auxiliary: e.auxiliary[d]}
			continue
		}
		if udocFiniteNullary(state) {
			if c.finiteNullary == nil {
				c.finiteNullary = make(map[ast.ASTHandle]rel.FiniteTable)
			}
			if c.finiteNullary[d] == nil {
				c.finiteOrder = append(c.finiteOrder, d)
			}
			c.finiteNullary[d] = state.table
			continue
		}
		relation := c.seeds[d]
		if relation == nil {
			var err error
			relation, err = c.factory.NewEmpty(state.sorts)
			if err != nil {
				return err
			}
			// Keep new owners reachable across a migration failure.
			c.seeds[d] = relation
			c.order = append(c.order, d)
		}
		if state.table != nil {
			facts, err := state.table.Facts()
			if err != nil {
				return err
			}
			for _, fact := range facts {
				if err = c.importFiniteFact(e, relation, state.sorts, fact); err != nil {
					return err
				}
			}
		}
		state.table = nil
		if c.finiteInputs[d] != nil {
			delete(c.finiteInputs, d)
			for i, declaration := range c.inputOrder {
				if declaration == d {
					c.inputOrder = append(c.inputOrder[:i], c.inputOrder[i+1:]...)
					break
				}
			}
		}
		// Go permits changing the requested representation before a query,
		// even though source set_predicate_kind requires no existing relation.
		// Preserve that extension by importing the nullary fact before retiring
		// its borrowed metadata; relation-manager storage still owns the table.
		if c.finiteNullary[d] != nil {
			delete(c.finiteNullary, d)
			for i, declaration := range c.finiteOrder {
				if declaration == d {
					c.finiteOrder = append(c.finiteOrder[:i], c.finiteOrder[i+1:]...)
					break
				}
			}
		}
	}
	return nil
}

func (c *datalogUDocContext) addFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if c.products != nil && c.products.program.productHead(declaration) && c.products.seeds[declaration] != nil {
		if err := c.products.addFact(declaration, values); err != nil {
			return err
		}
		return c.discardResult()
	}
	if input := c.finiteInputs[declaration]; input != nil {
		fact, err := c.relations.EncodeFact(input.sorts, values)
		if err != nil {
			return err
		}
		if err = c.discardResult(); err != nil {
			return err
		}
		_, err = input.table.Add(fact)
		return err
	}
	if table := c.finiteNullary[declaration]; table != nil {
		if len(values) != 0 {
			return ast.ErrASTDomain
		}
		if err := c.discardResult(); err != nil {
			return err
		}
		_, err := table.Add(nil)
		return err
	}
	relation := c.seeds[declaration]
	if relation == nil {
		return ErrUDocDatalogFragment
	}
	if err := c.discardResult(); err != nil {
		return err
	}
	_, err := relation.AddFact(values)
	return err
}

// hasSeedFacts exposes the source relation's pre-query contents to query-local
// transforms without moving UDoc ownership into the generic relation manager.
func (c *datalogUDocContext) hasSeedFacts(declaration ast.ASTHandle) (bool, error) {
	if c.products != nil && c.products.program.productHead(declaration) {
		if seed := c.products.seeds[declaration]; seed != nil {
			empty, err := seed.Empty()
			return !empty, err
		}
	}
	if input := c.finiteInputs[declaration]; input != nil {
		length, err := input.table.Len()
		return length != 0, err
	}
	if table := c.finiteNullary[declaration]; table != nil {
		return table.Contains(nil)
	}
	relation := c.seeds[declaration]
	if relation == nil {
		return false, nil
	}
	empty, err := relation.Empty()
	return !empty, err
}

func (c *datalogUDocContext) snapshot() error {
	if err := c.discardResult(); err != nil {
		return err
	}
	if err := c.products.snapshot(); err != nil {
		return err
	}
	c.result = make(map[ast.ASTHandle]*rel.TypedUDocRelation, len(c.seeds))
	c.nullaryResult = make(map[ast.ASTHandle]bool, len(c.finiteNullary))
	c.finiteResult = make(map[ast.ASTHandle]rel.FiniteTable, len(c.finiteInputs))
	for _, declaration := range c.inputOrder {
		copy, err := c.finiteInputs[declaration].table.CloneFinite()
		if copy != nil {
			c.finiteResult[declaration] = copy
		}
		if err != nil {
			return err
		}
	}
	for _, declaration := range c.finiteOrder {
		table := c.finiteNullary[declaration]
		present, err := table.Contains(nil)
		if err != nil {
			return err
		}
		c.nullaryResult[declaration] = present
	}
	for _, d := range c.order {
		copy, err := c.seeds[d].Clone()
		if err != nil {
			return err
		}
		c.result[d] = copy
	}
	return nil
}

func (c *datalogUDocContext) status(declarations []ast.ASTHandle) (lbool.LBool, error) {
	if len(declarations) == 0 {
		return lbool.True, nil
	}
	for _, d := range declarations {
		if c.products != nil && c.products.program.productHead(d) {
			empty, err := c.productEmpty(d)
			if err != nil {
				return lbool.Undef, err
			}
			if !empty {
				return lbool.True, nil
			}
			continue
		}
		if table := c.finiteResult[d]; table != nil {
			length, err := table.Len()
			if err != nil {
				return lbool.Undef, err
			}
			if length != 0 {
				return lbool.True, nil
			}
			continue
		}
		if present, found := c.nullaryResult[d]; found {
			if present {
				return lbool.True, nil
			}
			continue
		}
		r := c.result[d]
		if r == nil {
			return lbool.Undef, ErrUDocDatalogFragment
		}
		empty, err := r.Empty()
		if err != nil {
			return lbool.Undef, err
		}
		if !empty {
			return lbool.True, nil
		}
	}
	return lbool.False, nil
}

func (c *datalogUDocContext) answer(declaration ast.ASTHandle) (ast.ASTRef, error) {
	if c.products != nil && c.products.program.productHead(declaration) {
		return c.productAnswer(declaration)
	}
	if table := c.finiteResult[declaration]; table != nil {
		return c.finiteInputAnswer(declaration, table)
	}
	if result, found, err := c.nullaryAnswer(declaration); found {
		return result, err
	}
	r := c.result[declaration]
	if r == nil {
		return ast.ASTRef{}, ErrUDocDatalogFragment
	}
	return r.ToFormula()
}

func (c *datalogUDocContext) clone() (*datalogUDocContext, error) {
	copy := &datalogUDocContext{manager: c.manager, factory: c.factory, borrowed: true, seeds: make(map[ast.ASTHandle]*rel.TypedUDocRelation), order: append([]ast.ASTHandle(nil), c.order...)}
	copy.products = c.products.clone()
	for _, d := range c.order {
		relation, err := c.seeds[d].Clone()
		if err != nil {
			return copy, err
		}
		copy.seeds[d] = relation
	}
	return copy, nil
}
