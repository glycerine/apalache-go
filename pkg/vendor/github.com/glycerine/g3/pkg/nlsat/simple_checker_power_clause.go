package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1475-1532.

import (
	"errors"
	"github.com/glycerine/g3/pkg/sat"
)

func (c *simplePowerChecker) checkClause(clause *Clause) (consistent bool, err error) {
	type item struct {
		literal             sat.Literal
		form                simplePowerForm
		supported, possible bool
	}
	items := make([]item, 0, clause.Size())
	defer func() {
		for i := range items {
			if items[i].supported {
				err = errors.Join(err, items[i].form.close(c.solver.config.Algebraic))
			}
		}
	}()
	count, forced := 0, -1
	for _, literal := range clause.Literals() {
		form, supported, cause := c.solver.simplePowerLiteral(literal)
		if cause != nil {
			return false, cause
		}
		current := item{literal: literal, form: form, supported: supported, possible: true}
		if supported {
			current.possible, cause = c.possible(&current.form)
			if cause != nil {
				return false, cause
			}
			if !current.possible {
				c.learn(literal.Negated())
			}
		}
		items = append(items, current)
		if current.possible {
			count++
			forced = len(items) - 1
		}
	}
	if count == 0 {
		return false, nil
	}
	if count != 1 || !items[forced].supported {
		return true, nil
	}
	return c.force(&items[forced].form)
}
