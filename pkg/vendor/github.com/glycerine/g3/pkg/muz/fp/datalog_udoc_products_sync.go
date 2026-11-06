package fp

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (c *datalogUDocContext) syncProducts(e *PropositionalEngine, program *udocProductProgram) error {
	for d := range program.finite {
		if c.seeds[d] != nil {
			return ErrUDocDatalogFragment
		}
	}
	c.relations = e.relations
	if c.products == nil {
		c.products = &udocProductContext{}
	}
	if err := c.products.sync(e.relations, c.factory, program); err != nil {
		return err
	}
	// Rebuild borrowed input metadata so a previous finite-forwarding owner
	// cannot keep masking a predicate whose actual storage has become DOC.
	c.finiteInputs, c.inputOrder = nil, nil
	c.finiteNullary, c.finiteOrder = nil, nil
	for _, d := range e.predicateOrder {
		state := e.predicates[d]
		plan := program.rules[d]
		if program.productHead(d) {
			if c.products.seeds[d] != nil {
				state.table = nil
				continue
			}
			// Until a nonempty incoming register establishes product kind,
			// source store(empty) leaves the ordinary favourite finite owner.
			// Keep that actual owner available for subsequent head facts.
		}
		if _, mixed := program.groundProducts[d]; mixed {
			// Ground singleton assembly establishes this input's product kind
			// during saturation. Its initial ordinary table is empty and stays
			// engine-owned until that inferred product has been recorded.
			continue
		}
		if program.finite[d] || plan != nil && (plan.inputKind == 2 || program.productHead(d)) {
			if c.finiteInputs == nil {
				c.finiteInputs = make(map[ast.ASTHandle]*udocFiniteInput)
			}
			if c.finiteInputs[d] == nil {
				c.inputOrder = append(c.inputOrder, d)
			}
			c.finiteInputs[d] = &udocFiniteInput{table: state.table, sorts: append([]ast.ASTHandle(nil), state.sorts...), auxiliary: e.auxiliary[d]}
			continue
		}
		if !program.docs[d] && (plan == nil || plan.inputKind != 1) {
			return ErrUDocDatalogFragment
		}
		owner := c.seeds[d]
		if owner == nil {
			var err error
			owner, err = c.factory.NewEmpty(state.sorts)
			if owner != nil {
				c.seeds[d] = owner
				c.order = append(c.order, d)
			}
			if err != nil {
				return err
			}
		}
		if state.table != nil {
			facts, err := state.table.Facts()
			if err != nil {
				return err
			}
			for _, fact := range facts {
				if err = c.importFiniteFact(e, owner, state.sorts, fact); err != nil {
					return err
				}
			}
		}
		state.table = nil
	}
	return nil
}

func (e *PropositionalEngine) selectUDocProducts(owner *datalogUDocContext, favourite bool, checkerEnabled bool) (bool, error) {
	// AddFact before the first query must still populate ordinary finite
	// heads. Nonempty ordinary heads are not eligible for source kind inference.
	if owner.factory.FormulaChecked() || !e.queried && owner.products == nil {
		return false, nil
	}
	var known map[ast.ASTHandle][]ast.ASTHandle
	if owner.products != nil {
		known = owner.products.program.sorts
	}
	program, err := e.planUDocProducts(owner.factory, favourite, known)
	if err != nil {
		return false, err
	}
	if program == nil {
		if owner.products != nil {
			return true, ErrUDocDatalogFragment
		}
		return false, nil
	}
	if checkerEnabled {
		return false, nil
	}
	if enabled, err := e.datalogExplanationsEnabled(); err != nil || enabled {
		return false, err
	}
	if _, enabled, err := e.datalogDefaultTableCheck(); err != nil || enabled {
		return false, err
	}
	e.udocDatalog = owner
	return true, owner.syncProducts(e, program)
}

func (c *datalogUDocContext) syncExistingProducts(e *PropositionalEngine) error {
	program, err := e.planUDocProducts(c.factory, true, c.products.program.sorts)
	if err != nil || program == nil {
		return errors.Join(err, ErrUDocDatalogFragment)
	}
	return c.syncProducts(e, program)
}

func (c *datalogUDocContext) saturateProducts(set *base.RuleSet) error {
	// The source compiler runs extensional ground-rule strata before their
	// consumers, irrespective of where those rules occurred in the input.
	// Populate this query's snapshots; rule facts never become API seed facts.
	if err := c.evaluateProductGroundFacts(c.products.program.groundFacts); err != nil {
		return err
	}
	for _, d := range c.products.program.order {
		if err := c.checkpoint(); err != nil {
			return err
		}
		plan := c.products.program.rules[d]
		if plan.inputKind == 1 {
			rule, err := set.Rule(plan.ruleIndex)
			if err != nil {
				return err
			}
			if _, err = c.evaluateRule(rule); err != nil {
				return err
			}
			continue
		}
		if plan.inputKind == 2 {
			if err := c.evaluateProductFiniteQuery(plan); err != nil {
				return err
			}
			continue
		}
		if !plan.source.IsNull() && c.products.result[plan.source] == nil && c.finiteResult[plan.source] != nil {
			if err := c.evaluateProductFiniteQuery(plan); err != nil {
				return err
			}
			continue
		}
		if err := c.products.evaluate(plan, c.result, c.finiteResult); err != nil {
			return err
		}
	}
	return nil
}

// Source product::empty is deliberately component-coarse even though source
// DOC and sieve report is_precise=true. Query status follows that behavior;
// ToFormula remains the actual conjunction and may therefore be unsatisfiable.
func (c *datalogUDocContext) productEmpty(d ast.ASTHandle) (bool, error) {
	if value := c.products.result[d]; value != nil {
		return value.Empty()
	}
	if table := c.finiteResult[d]; table != nil {
		count, err := table.Len()
		return count == 0, err
	}
	return true, nil
}

func (c *datalogUDocContext) productAnswer(d ast.ASTHandle) (ast.ASTRef, error) {
	if value := c.products.result[d]; value != nil {
		return value.ToFormula()
	}
	if table := c.finiteResult[d]; table != nil {
		return c.finiteInputAnswer(d, table)
	}
	value, err := c.manager.MkFalse()
	if err != nil {
		return ast.ASTRef{}, err
	}
	return c.manager.NewRef(value)
}
