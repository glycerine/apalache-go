package smtcore

// Source: src/smt/theory_arith_nl.h mark_var, mark_dependents, and
// get_non_linear_cluster. The replay tableau has ordinary base rows only.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
)

type smtNonlinearCluster struct {
	columns []uint32
	rows    []simplex.Row
}

func (t *SMTLinearArithmeticTheory) nonlinearCluster(solver *simplex.Solver) (smtNonlinearCluster, error) {
	var result smtNonlinearCluster
	found := make(map[uint32]bool)
	visited := make(map[simplex.Row]bool)
	mark := func(column uint32) {
		if !found[column] {
			found[column] = true
			result.columns = append(result.columns, column)
		}
	}
	for _, column := range t.nonlinearColumns {
		variable, exists := t.theoryVariable(column)
		if !exists {
			return result, ast.ErrASTDomain
		}
		node, err := t.Enode(variable)
		if err != nil {
			return result, err
		}
		if t.state.relevancy == nil || t.state.relevancy.IsRelevant(node.owner) {
			mark(column)
		}
	}
	for i := 0; i < len(result.columns); i++ {
		column := result.columns[i]
		for _, factor := range t.nonlinearFactors[column] {
			mark(factor)
		}
		if _, fixed := smtFixedSimplexColumn(solver, column); fixed {
			continue
		}
		for _, row := range solver.ColumnRows(column) {
			if visited[row] {
				continue
			}
			visited[row] = true
			base, err := solver.BaseVariable(row)
			if err != nil {
				return result, err
			}
			_, lower := solver.Lower(base)
			_, upper := solver.Upper(base)
			if !lower && !upper && base != column {
				continue
			}
			result.rows = append(result.rows, row)
			entries, err := solver.RowEntries(row)
			if err != nil {
				return result, err
			}
			for _, entry := range entries {
				if _, fixed := smtFixedSimplexColumn(solver, entry.Variable); !fixed {
					mark(entry.Variable)
				}
			}
		}
	}
	// init_grobner also consumes each clustered variable's own base row,
	// including a row that traversal could omit as a free substitution.
	rows := make(map[simplex.Row]bool, len(result.rows))
	for _, row := range result.rows {
		rows[row] = true
	}
	for _, column := range result.columns {
		if row, basic := solver.BasicRow(column); basic && !rows[row] {
			rows[row] = true
			result.rows = append(result.rows, row)
		}
	}
	return result, nil
}
