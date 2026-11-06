package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1475-1532.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

func (c *simpleLinearChecker) checkClause(clause *Clause) (consistent bool, err error) {
	type candidate struct {
		literal             sat.Literal
		form                simpleLinearForm
		supported, possible bool
	}
	items := make([]candidate, 0, clause.Size())
	defer func() {
		for i := range items {
			if items[i].supported {
				err = errors.Join(err, items[i].form.close(c.solver.config.Algebraic))
			}
		}
	}()
	possibleCount, forced := -0, -1
	for _, literal := range clause.Literals() {
		form, supported, cause := c.form(literal)
		if cause != nil {
			return false, cause
		}
		item := candidate{literal: literal, form: form, supported: supported, possible: true}
		if supported {
			item.possible, cause = c.possible(&item.form)
			if cause != nil {
				return false, cause
			}
			if !item.possible {
				c.learn(literal.Negated())
			}
		}
		items = append(items, item)
		if item.possible {
			possibleCount++
			forced = len(items) - 1
		}
	}
	if possibleCount == 0 {
		return false, nil
	}
	if possibleCount != 1 || !items[forced].supported {
		return true, nil
	}
	return c.force(&items[forced].form)
}
