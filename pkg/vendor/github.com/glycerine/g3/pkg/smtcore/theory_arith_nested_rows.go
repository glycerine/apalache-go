package smtcore

// Source: src/smt/theory_arith_nl.h is_problematic_non_linear_row and
// is_cross_nested_consistent(row), is_cross_nested_consistent(nl_cluster).

import (
	"errors"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

func (n *smtNestedArithmetic) columnInteger(column uint32) bool {
	if column == 0 || n.theory.integer || n.theory.integerColumns[column] {
		return true
	}
	if column >= n.theory.nextSimplex {
		index := column - n.theory.nextSimplex
		if int(index) < len(n.theory.atoms) {
			return n.theory.integralAtom(&n.theory.atoms[index])
		}
	}
	return false
}

func (n *smtNestedArithmetic) problematicRow(entries []simplex.RowEntry) bool {
	seen := make(map[uint32]bool)
	for _, entry := range entries {
		column := entry.Variable
		if smtArithmeticColumnFixed(n.solver, column) {
			continue
		}
		if factors := n.theory.nonlinearFactors[column]; len(factors) != 0 {
			for _, factor := range factors {
				if seen[factor] {
					return true
				}
			}
			for _, factor := range factors {
				if !smtArithmeticColumnFixed(n.solver, factor) {
					seen[factor] = true
				}
			}
		} else {
			if seen[column] {
				return true
			}
			seen[column] = true
		}
	}
	return false
}

func (n *smtNestedArithmetic) rowForm(entries []simplex.RowEntry) (smtLinearForm, error) {
	form := newSMTLinearForm()
	for _, entry := range entries {
		column := entry.Variable
		if column == 0 {
			form.constant.Add(&form.constant, entry.Coefficient)
			continue
		}
		if _, found := n.theory.theoryVariable(column); !found {
			if n.aliases == nil {
				n.aliases = make(map[uint32]ast.ASTHandle)
				n.columns = make(map[ast.ASTHandle]uint32)
			}
			if _, exists := n.aliases[column]; !exists {
				kind := int32(0)
				if n.columnInteger(column) {
					kind = 1
				}
				sort, err := n.theory.state.manager.MkFamilySort(n.theory.FamilyID(), kind, nil)
				if err != nil {
					return form, err
				}
				alias, err := n.refs.hold(n.theory.state.manager.MkVar(column, sort))
				if err != nil {
					return form, err
				}
				n.aliases[column], n.columns[alias] = alias, column
			}
		}
		form.addTerm(column, entry.Coefficient)
	}
	return form, nil
}

func (n *smtNestedArithmetic) checkClusterRows() (bool, error) {
	cluster, err := n.theory.nonlinearCluster(n.solver)
	if err != nil {
		return false, err
	}
	for _, column := range cluster.columns {
		row, basic := n.solver.BasicRow(column)
		if !basic {
			continue
		}
		n.theory.statistics.crossNested++
		entries, err := n.solver.RowEntries(row)
		if err != nil {
			return false, err
		}
		if !n.problematicRow(entries) {
			continue
		}
		form, err := n.rowForm(entries)
		if err != nil {
			return false, err
		}
		polynomial, supported, err := n.terms(&form)
		if err != nil {
			return false, err
		}
		if !supported {
			continue
		}
		variables := n.variables(polynomial)
		sort.SliceStable(variables, func(i, j int) bool { return variables[i].count > variables[j].count })
		for _, variable := range variables {
			interval, err := n.nestedInterval(polynomial, variable.expression)
			if err != nil {
				return false, err
			}
			// Every canonical row is an equality to zero. Its interval must
			// contain zero, including open endpoint information.
			if interval.positiveOne() || interval.negativeOne() {
				return true, n.theory.arithmeticIntervalConflict(interval, interval.positiveOne())
			}
		}
	}
	return false, nil
}

// Source resets m_nl_new_exprs before nesting each variable. Polynomial
// inputs and private column aliases stay pinned for the enclosing check.
func (n *smtNestedArithmetic) nestedInterval(polynomial []smtNestedTerm, variable ast.ASTHandle) (interval *smtOldInterval, result error) {
	limit := len(n.refs.refs)
	defer func() {
		temporary := smtArithmeticDivisionCall{theory: n.theory, refs: n.refs.refs[limit:]}
		result = errors.Join(result, temporary.close())
		n.refs.refs = n.refs.refs[:limit]
	}()
	expression, err := n.crossNested(polynomial, variable)
	if err != nil {
		return nil, err
	}
	return n.evaluate(expression)
}
