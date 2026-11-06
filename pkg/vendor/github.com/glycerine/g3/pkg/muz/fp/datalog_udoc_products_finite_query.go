package fp

// Source: ordinary finite select/filter/project/rename query instructions.
// A mixed program keeps its extensional finite predicate queryable through
// its actual table owner; these rows never decide a product-head answer.

import "github.com/glycerine/g3/pkg/muz/rel"

func (c *datalogUDocContext) evaluateProductFiniteQuery(plan *udocProductRulePlan) error {
	source, target := c.finiteResult[plan.source], c.finiteResult[plan.head]
	input := c.finiteInputs[plan.source]
	if source == nil || target == nil || input == nil {
		return ErrUDocDatalogFragment
	}
	constants := make([]rel.Element, len(plan.filters))
	for i, filter := range plan.filters {
		value, err := c.relations.EncodeValue(input.sorts[filter.column], filter.value)
		if err != nil {
			return err
		}
		constants[i] = value
	}
	facts, err := source.Facts()
	if err != nil {
		return err
	}
	for _, fact := range facts {
		matches := true
		for i, filter := range plan.filters {
			matches = matches && fact[filter.column] == constants[i]
		}
		for _, columns := range plan.identical {
			matches = matches && fact[columns[0]] == fact[columns[1]]
		}
		if !matches {
			continue
		}
		row := make(rel.Fact, len(plan.output))
		for i, column := range plan.output {
			row[i] = fact[column]
		}
		if _, err = target.Add(row); err != nil {
			return err
		}
	}
	return nil
}
