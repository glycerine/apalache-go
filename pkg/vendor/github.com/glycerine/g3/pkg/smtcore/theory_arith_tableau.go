package smtcore

// Source: src/smt/theory_arith.h:97-216 and
// src/smt/theory_arith_aux.h:29-330.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

const smtArithmeticDeadRow = -1

type smtArithmeticRowEntry struct {
	coefficient big.Rat
	variable    SMTTheoryVar
	columnIndex int
	freeNext    int
}

func (e *smtArithmeticRowEntry) dead() bool { return e.variable == SMTNullTheoryVar }

type smtArithmeticColumnEntry struct {
	rowID    int
	rowIndex int
	freeNext int
}

func (e *smtArithmeticColumnEntry) dead() bool { return e.rowID == smtArithmeticDeadRow }

type smtArithmeticRow struct {
	entries   []smtArithmeticRowEntry
	size      int
	base      SMTTheoryVar
	firstFree int
}

func newSMTArithmeticRow() smtArithmeticRow {
	return smtArithmeticRow{base: SMTNullTheoryVar, firstFree: -1}
}

func (r *smtArithmeticRow) reset() {
	r.entries = r.entries[:0]
	r.size, r.base, r.firstFree = 0, SMTNullTheoryVar, -1
}

func (r *smtArithmeticRow) addEntry() (int, *smtArithmeticRowEntry) {
	r.size++
	if r.firstFree < 0 {
		index := len(r.entries)
		r.entries = append(r.entries, smtArithmeticRowEntry{variable: SMTNullTheoryVar, freeNext: -1})
		return index, &r.entries[index]
	}
	index := r.firstFree
	entry := &r.entries[index]
	r.firstFree = entry.freeNext
	entry.freeNext = -1
	return index, entry
}

func (r *smtArithmeticRow) deleteEntry(index int) error {
	if index < 0 || index >= len(r.entries) || r.entries[index].dead() {
		return ast.ErrASTDomain
	}
	entry := &r.entries[index]
	entry.variable, entry.freeNext = SMTNullTheoryVar, r.firstFree
	entry.coefficient.SetInt64(0)
	r.firstFree, r.size = index, r.size-1
	return nil
}

func (r *smtArithmeticRow) compress(columns []smtArithmeticColumn) error {
	target := 0
	for source := range r.entries {
		entry := &r.entries[source]
		if entry.dead() {
			continue
		}
		if int(entry.variable) < 0 || int(entry.variable) >= len(columns) || entry.columnIndex < 0 || entry.columnIndex >= len(columns[entry.variable].entries) {
			return ast.ErrASTDomain
		}
		if source != target {
			r.entries[target] = *entry
			r.entries[target].coefficient.Set(&entry.coefficient)
			columns[entry.variable].entries[entry.columnIndex].rowIndex = target
		}
		target++
	}
	if target != r.size {
		return ast.ErrASTDomain
	}
	for index := target; index < len(r.entries); index++ {
		r.entries[index] = smtArithmeticRowEntry{}
	}
	r.entries, r.firstFree = r.entries[:target], -1
	return nil
}

func (r *smtArithmeticRow) compressIfNeeded(columns []smtArithmeticColumn) error {
	if r.size*2 < len(r.entries) {
		return r.compress(columns)
	}
	return nil
}

func (r *smtArithmeticRow) saveVariablePositions(positions []int) error {
	for index := range r.entries {
		entry := &r.entries[index]
		if entry.dead() {
			continue
		}
		if int(entry.variable) < 0 || int(entry.variable) >= len(positions) {
			return ast.ErrASTDomain
		}
		positions[entry.variable] = index
	}
	return nil
}

func (r *smtArithmeticRow) resetVariablePositions(positions []int) error {
	for index := range r.entries {
		entry := &r.entries[index]
		if entry.dead() {
			continue
		}
		if int(entry.variable) < 0 || int(entry.variable) >= len(positions) {
			return ast.ErrASTDomain
		}
		positions[entry.variable] = -1
	}
	return nil
}

func (r *smtArithmeticRow) denominatorLCM() *big.Int {
	result := big.NewInt(1)
	for index := range r.entries {
		entry := &r.entries[index]
		if !entry.dead() {
			result = smtRatLCM(result, entry.coefficient.Denom())
		}
	}
	return result
}

func (r *smtArithmeticRow) indexOf(variable SMTTheoryVar) int {
	for index := range r.entries {
		if !r.entries[index].dead() && r.entries[index].variable == variable {
			return index
		}
	}
	return -1
}

type smtArithmeticColumn struct {
	entries   []smtArithmeticColumnEntry
	size      int
	firstFree int
}

type smtArithmeticTableau struct {
	rows      []smtArithmeticRow
	columns   []smtArithmeticColumn
	rowScopes []map[SMTTheoryVar]bool
}

func (t *smtArithmeticTableau) addVariable() SMTTheoryVar {
	variable := SMTTheoryVar(len(t.columns))
	t.columns = append(t.columns, newSMTArithmeticColumn())
	return variable
}

func (t *smtArithmeticTableau) makeRow() int {
	row := len(t.rows)
	t.rows = append(t.rows, newSMTArithmeticRow())
	return row
}

func (t *smtArithmeticTableau) pushRowScope() {
	t.rowScopes = append(t.rowScopes, make(map[SMTTheoryVar]bool))
}
func (t *smtArithmeticTableau) popRowScope() bool {
	if len(t.rowScopes) == 0 {
		return false
	}
	clear(t.rowScopes[len(t.rowScopes)-1])
	t.rowScopes = t.rowScopes[:len(t.rowScopes)-1]
	return true
}

func (t *smtArithmeticTableau) addRowEntry(rowID int, coefficient *big.Rat, variable SMTTheoryVar, invert bool) error {
	if rowID < 0 || rowID >= len(t.rows) || coefficient == nil || variable < 0 || int(variable) >= len(t.columns) || len(t.rowScopes) == 0 {
		return ast.ErrASTDomain
	}
	row, column := &t.rows[rowID], &t.columns[variable]
	variables := t.rowScopes[len(t.rowScopes)-1]
	if variables[variable] {
		index := row.indexOf(variable)
		if index < 0 {
			return ast.ErrASTDomain
		}
		if invert {
			row.entries[index].coefficient.Sub(&row.entries[index].coefficient, coefficient)
		} else {
			row.entries[index].coefficient.Add(&row.entries[index].coefficient, coefficient)
		}
		return nil
	}
	variables[variable] = true
	rowIndex, rowEntry := row.addEntry()
	columnIndex, columnEntry := column.addEntry()
	rowEntry.variable, rowEntry.columnIndex = variable, columnIndex
	rowEntry.coefficient.Set(coefficient)
	if invert {
		rowEntry.coefficient.Neg(&rowEntry.coefficient)
	}
	columnEntry.rowID, columnEntry.rowIndex = rowID, rowIndex
	return nil
}

func newSMTArithmeticColumn() smtArithmeticColumn { return smtArithmeticColumn{firstFree: -1} }

func (c *smtArithmeticColumn) reset() {
	c.entries = c.entries[:0]
	c.size, c.firstFree = 0, -1
}

func (c *smtArithmeticColumn) addEntry() (int, *smtArithmeticColumnEntry) {
	c.size++
	if c.firstFree < 0 {
		index := len(c.entries)
		c.entries = append(c.entries, smtArithmeticColumnEntry{rowID: smtArithmeticDeadRow, freeNext: -1})
		return index, &c.entries[index]
	}
	index := c.firstFree
	entry := &c.entries[index]
	c.firstFree = entry.freeNext
	entry.freeNext = -1
	return index, entry
}

func (c *smtArithmeticColumn) deleteEntry(index int) error {
	if index < 0 || index >= len(c.entries) || c.entries[index].dead() {
		return ast.ErrASTDomain
	}
	entry := &c.entries[index]
	entry.rowID, entry.freeNext = smtArithmeticDeadRow, c.firstFree
	c.firstFree, c.size = index, c.size-1
	return nil
}

func (c *smtArithmeticColumn) compress(rows []smtArithmeticRow) error {
	target := 0
	for source := range c.entries {
		entry := c.entries[source]
		if entry.dead() {
			continue
		}
		if entry.rowID < 0 || entry.rowID >= len(rows) || entry.rowIndex < 0 || entry.rowIndex >= len(rows[entry.rowID].entries) {
			return ast.ErrASTDomain
		}
		if source != target {
			c.entries[target] = entry
			rows[entry.rowID].entries[entry.rowIndex].columnIndex = target
		}
		target++
	}
	if target != c.size {
		return ast.ErrASTDomain
	}
	clear(c.entries[target:])
	c.entries, c.firstFree = c.entries[:target], -1
	return nil
}

func (c *smtArithmeticColumn) compressIfNeeded(rows []smtArithmeticRow) error {
	if c.size*2 < len(c.entries) {
		return c.compress(rows)
	}
	return nil
}

func (c *smtArithmeticColumn) compressSingleton(rows []smtArithmeticRow, singleton int) error {
	if c.size != 1 || singleton < 0 || singleton >= len(c.entries) || c.entries[singleton].dead() {
		return ast.ErrASTDomain
	}
	entry := c.entries[singleton]
	if entry.rowID < 0 || entry.rowID >= len(rows) || entry.rowIndex < 0 || entry.rowIndex >= len(rows[entry.rowID].entries) {
		return ast.ErrASTDomain
	}
	if singleton != 0 {
		c.entries[0] = entry
		rows[entry.rowID].entries[entry.rowIndex].columnIndex = 0
	}
	clear(c.entries[1:])
	c.entries, c.firstFree = c.entries[:1], -1
	return nil
}

func (c *smtArithmeticColumn) firstEntry() *smtArithmeticColumnEntry {
	for index := range c.entries {
		if !c.entries[index].dead() {
			return &c.entries[index]
		}
	}
	return nil
}
