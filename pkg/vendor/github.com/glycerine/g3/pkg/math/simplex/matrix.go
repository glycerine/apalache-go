package simplex

// Package simplex implements the retained exact sparse tableau and simplex
// engines from src/math/simplex.

import (
	"fmt"
	"io"
	"math/big"
)

const NullVariable = ^uint32(0)

// Row is a stable matrix row identifier. Deleted identifiers are recycled by
// the next NewRow call, as in sparse_matrix::mk_row.
type Row struct{ id uint32 }

func (r Row) ID() uint32 { return r.id }

// RowEntry is a borrowed coefficient-variable pair returned by Entries.
type RowEntry struct {
	Variable    uint32
	Coefficient *big.Rat
}

type matrixEntry struct {
	variable uint32
	coeff    big.Rat
}
type matrixRow struct {
	entries []matrixEntry
	live    bool
}

// Matrix is a concrete exact-rational replacement for the two retained C++
// sparse_matrix instantiations. Integer coefficients use denominator one.
type Matrix struct {
	rows    []matrixRow
	dead    []uint32
	columns [][]uint32
	addRows uint32
}

func (m *Matrix) EnsureVariable(v uint32) {
	for uint32(len(m.columns)) <= v {
		m.columns = append(m.columns, nil)
	}
}

func (m *Matrix) NewRow() Row {
	if len(m.dead) != 0 {
		i := m.dead[len(m.dead)-1]
		m.dead = m.dead[:len(m.dead)-1]
		m.rows[i] = matrixRow{live: true}
		return Row{i}
	}
	r := Row{uint32(len(m.rows))}
	m.rows = append(m.rows, matrixRow{live: true})
	return r
}

func (m *Matrix) valid(r Row) bool { return int(r.id) < len(m.rows) && m.rows[r.id].live }

func (m *Matrix) addColumn(v, row uint32) {
	m.EnsureVariable(v)
	for _, r := range m.columns[v] {
		if r == row {
			return
		}
	}
	m.columns[v] = append(m.columns[v], row)
}
func (m *Matrix) delColumn(v, row uint32) {
	if int(v) >= len(m.columns) {
		return
	}
	c := m.columns[v]
	for i, r := range c {
		if r == row {
			copy(c[i:], c[i+1:])
			m.columns[v] = c[:len(c)-1]
			return
		}
	}
}

func (m *Matrix) AddRational(r Row, coefficient *big.Rat, variable uint32) error {
	if !m.valid(r) || coefficient == nil || coefficient.Sign() == 0 {
		return fmt.Errorf("simplex: invalid matrix insertion")
	}
	m.EnsureVariable(variable)
	row := &m.rows[r.id]
	for i := range row.entries {
		if row.entries[i].variable == variable {
			return fmt.Errorf("simplex: duplicate variable %d", variable)
		}
	}
	var e matrixEntry
	e.variable = variable
	e.coeff.Set(coefficient)
	row.entries = append(row.entries, e)
	m.addColumn(variable, r.id)
	return nil
}

func (m *Matrix) AddInteger(r Row, coefficient *big.Int, variable uint32) error {
	if coefficient == nil {
		return fmt.Errorf("simplex: nil coefficient")
	}
	return m.AddRational(r, new(big.Rat).SetInt(coefficient), variable)
}

func (m *Matrix) Entries(r Row) ([]RowEntry, error) {
	if !m.valid(r) {
		return nil, fmt.Errorf("simplex: invalid row")
	}
	out := make([]RowEntry, len(m.rows[r.id].entries))
	for i := range out {
		out[i] = RowEntry{m.rows[r.id].entries[i].variable, new(big.Rat).Set(&m.rows[r.id].entries[i].coeff)}
	}
	return out, nil
}

func (m *Matrix) coefficient(r Row, variable uint32) (*big.Rat, bool) {
	if !m.valid(r) {
		return nil, false
	}
	for i := range m.rows[r.id].entries {
		if m.rows[r.id].entries[i].variable == variable {
			return &m.rows[r.id].entries[i].coeff, true
		}
	}
	return nil, false
}

// Add sets destination = destination + coefficient*source.
func (m *Matrix) Add(destination Row, coefficient *big.Rat, source Row) error {
	if !m.valid(destination) || !m.valid(source) || coefficient == nil {
		return fmt.Errorf("simplex: invalid row addition")
	}
	m.addRows++
	if coefficient.Sign() == 0 {
		return nil
	}
	snapshot := append([]matrixEntry(nil), m.rows[source.id].entries...)
	for _, item := range snapshot {
		term := new(big.Rat).Mul(&item.coeff, coefficient)
		current, found := m.coefficient(destination, item.variable)
		if !found {
			if err := m.AddRational(destination, term, item.variable); err != nil {
				return err
			}
			continue
		}
		current.Add(current, term)
		if current.Sign() == 0 {
			m.removeVariable(destination, item.variable)
		}
	}
	return nil
}

func (m *Matrix) removeVariable(r Row, variable uint32) {
	row := &m.rows[r.id]
	for i := range row.entries {
		if row.entries[i].variable == variable {
			copy(row.entries[i:], row.entries[i+1:])
			row.entries = row.entries[:len(row.entries)-1]
			m.delColumn(variable, r.id)
			return
		}
	}
}

func (m *Matrix) Multiply(r Row, coefficient *big.Rat) error {
	if !m.valid(r) || coefficient == nil || coefficient.Sign() == 0 {
		return fmt.Errorf("simplex: invalid row multiplier")
	}
	if coefficient.Cmp(big.NewRat(1, 1)) == 0 {
		return nil
	}
	for i := range m.rows[r.id].entries {
		m.rows[r.id].entries[i].coeff.Mul(&m.rows[r.id].entries[i].coeff, coefficient)
	}
	return nil
}
func (m *Matrix) Negate(r Row) error { return m.Multiply(r, big.NewRat(-1, 1)) }

func (m *Matrix) Delete(r Row) error {
	if !m.valid(r) {
		return fmt.Errorf("simplex: invalid row")
	}
	for _, e := range m.rows[r.id].entries {
		m.delColumn(e.variable, r.id)
	}
	m.rows[r.id] = matrixRow{}
	m.dead = append(m.dead, r.id)
	return nil
}

func gcdInt(a, b *big.Int) *big.Int {
	a = new(big.Int).Abs(new(big.Int).Set(a))
	b = new(big.Int).Abs(new(big.Int).Set(b))
	return new(big.Int).GCD(nil, nil, a, b)
}
func lcmInt(a, b *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Int)
	}
	g := gcdInt(a, b)
	return new(big.Int).Abs(new(big.Int).Mul(new(big.Int).Quo(new(big.Int).Set(a), g), b))
}

// GCDNormalize divides a row by the exact rational gcd of its coefficients
// and returns that gcd. For integral rows this is the ordinary positive GCD.
func (m *Matrix) GCDNormalize(r Row) (*big.Rat, error) {
	if !m.valid(r) {
		return nil, fmt.Errorf("simplex: invalid row")
	}
	row := &m.rows[r.id]
	if len(row.entries) == 0 {
		return big.NewRat(1, 1), nil
	}
	num := new(big.Int)
	den := big.NewInt(1)
	for i := range row.entries {
		c := &row.entries[i].coeff
		if num.Sign() == 0 {
			num.Abs(c.Num())
		} else {
			num = gcdInt(num, c.Num())
		}
		den = lcmInt(den, c.Denom())
	}
	if num.Sign() == 0 {
		return big.NewRat(1, 1), nil
	}
	g := new(big.Rat).SetFrac(num, den)
	if g.Cmp(big.NewRat(1, 1)) != 0 {
		for i := range row.entries {
			row.entries[i].coeff.Quo(&row.entries[i].coeff, g)
		}
	}
	return g, nil
}

func (m *Matrix) ColumnSize(variable uint32) uint32 {
	if int(variable) >= len(m.columns) {
		return 0
	}
	return uint32(len(m.columns[variable]))
}
func (m *Matrix) ColumnRows(variable uint32) []Row {
	if int(variable) >= len(m.columns) {
		return nil
	}
	out := make([]Row, len(m.columns[variable]))
	for i, r := range m.columns[variable] {
		out[i] = Row{r}
	}
	return out
}
func (m *Matrix) AddRowCount() uint32 { return m.addRows }

func (m *Matrix) Reset() { *m = Matrix{} }

func (m *Matrix) WellFormed() bool {
	for rid := range m.rows {
		if !m.rows[rid].live {
			continue
		}
		seen := make(map[uint32]bool)
		for i := range m.rows[rid].entries {
			e := &m.rows[rid].entries[i]
			if e.coeff.Sign() == 0 || seen[e.variable] || int(e.variable) >= len(m.columns) {
				return false
			}
			seen[e.variable] = true
			found := false
			for _, r := range m.columns[e.variable] {
				if int(r) == rid {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}
	for v, rows := range m.columns {
		seen := make(map[uint32]bool)
		for _, rid := range rows {
			if seen[rid] || int(rid) >= len(m.rows) || !m.rows[rid].live {
				return false
			}
			seen[rid] = true
			if _, ok := m.coefficient(Row{rid}, uint32(v)); !ok {
				return false
			}
		}
	}
	return true
}

func (m *Matrix) DisplayRow(w io.Writer, r Row) error {
	entries, err := m.Entries(r)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if _, err = fmt.Fprintf(w, "%s*v%d ", e.Coefficient.RatString(), e.Variable); err != nil {
			return err
		}
	}
	_, err = io.WriteString(w, "\n")
	return err
}
func (m *Matrix) Display(w io.Writer) error {
	for i := range m.rows {
		if m.rows[i].live && len(m.rows[i].entries) != 0 {
			if err := m.DisplayRow(w, Row{uint32(i)}); err != nil {
				return err
			}
		}
	}
	return nil
}
