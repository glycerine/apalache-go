package simplex

import (
	"fmt"
	"io"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// Infinitesimal is the exact c*infinity+a+b*epsilon value. Infinity is zero
// for the existing mpz/mpq consumers; the retained arithmetic inf_ext
// specialization uses all three rational components, ordered lexicographically.
type Infinitesimal struct{ Infinity, Main, Epsilon big.Rat }

func NewInfinitesimal(main, epsilon *big.Rat) Infinitesimal {
	return NewInfiniteInfinitesimal(nil, main, epsilon)
}
func NewInfiniteInfinitesimal(infinity, main, epsilon *big.Rat) Infinitesimal {
	var r Infinitesimal
	if infinity != nil {
		r.Infinity.Set(infinity)
	}
	if main != nil {
		r.Main.Set(main)
	}
	if epsilon != nil {
		r.Epsilon.Set(epsilon)
	}
	return r
}
func RationalInfinitesimal(v *big.Rat) Infinitesimal { return NewInfinitesimal(v, nil) }
func (a *Infinitesimal) Set(b *Infinitesimal) {
	a.Infinity.Set(&b.Infinity)
	a.Main.Set(&b.Main)
	a.Epsilon.Set(&b.Epsilon)
}
func (a *Infinitesimal) Clone() Infinitesimal {
	return NewInfiniteInfinitesimal(&a.Infinity, &a.Main, &a.Epsilon)
}
func (a *Infinitesimal) Add(b *Infinitesimal) {
	a.Infinity.Add(&a.Infinity, &b.Infinity)
	a.Main.Add(&a.Main, &b.Main)
	a.Epsilon.Add(&a.Epsilon, &b.Epsilon)
}
func (a *Infinitesimal) Sub(b *Infinitesimal) {
	a.Infinity.Sub(&a.Infinity, &b.Infinity)
	a.Main.Sub(&a.Main, &b.Main)
	a.Epsilon.Sub(&a.Epsilon, &b.Epsilon)
}
func (a *Infinitesimal) Neg() {
	a.Infinity.Neg(&a.Infinity)
	a.Main.Neg(&a.Main)
	a.Epsilon.Neg(&a.Epsilon)
}
func (a *Infinitesimal) Mul(q *big.Rat) {
	a.Infinity.Mul(&a.Infinity, q)
	a.Main.Mul(&a.Main, q)
	a.Epsilon.Mul(&a.Epsilon, q)
}
func (a *Infinitesimal) Quo(q *big.Rat) {
	a.Infinity.Quo(&a.Infinity, q)
	a.Main.Quo(&a.Main, q)
	a.Epsilon.Quo(&a.Epsilon, q)
}
func (a *Infinitesimal) Sign() int {
	if n := a.Infinity.Sign(); n != 0 {
		return n
	}
	if n := a.Main.Sign(); n != 0 {
		return n
	}
	return a.Epsilon.Sign()
}
func (a *Infinitesimal) Cmp(b *Infinitesimal) int {
	if n := a.Infinity.Cmp(&b.Infinity); n != 0 {
		return n
	}
	if n := a.Main.Cmp(&b.Main); n != 0 {
		return n
	}
	return a.Epsilon.Cmp(&b.Epsilon)
}
func (a *Infinitesimal) IsZero() bool {
	return a.Infinity.Sign() == 0 && a.Main.Sign() == 0 && a.Epsilon.Sign() == 0
}
func (a *Infinitesimal) IsRational() bool { return a.Infinity.Sign() == 0 && a.Epsilon.Sign() == 0 }
func (a *Infinitesimal) Abs() {
	if a.Sign() < 0 {
		a.Neg()
	}
}
func (a *Infinitesimal) String() string {
	text := a.Main.RatString()
	if a.Epsilon.Sign() != 0 {
		text += "+" + a.Epsilon.RatString() + "*epsilon"
	}
	if a.Infinity.Sign() != 0 {
		text = a.Infinity.RatString() + "*infinity+" + text
	}
	return text
}

type variableInfo struct {
	row                                  uint32
	base, lowerValid, upperValid         bool
	value, lower, upper                  Infinitesimal
	baseCoeff                            big.Rat
	lowerDependencies, upperDependencies []BoundDependency
	lowerExplained, upperExplained       bool
}

type Stats struct{ NumPivots, NumInfeasible, NumChecks uint32 }

// Solver is the concrete exact tableau used by both retained mpz and mpq
// consumers. Integer rows are accepted through AddIntegerRow.
type Solver struct {
	arithmeticBoundAtoms     map[arithmeticBoundKey][]arithmeticBoundAtom
	boundConflict            []BoundDependency
	boundConflictKnown       bool
	limit                    *control.Reslimit
	matrix                   Matrix
	vars                     []variableInfo
	rowBase                  []uint32
	patch                    map[uint32]bool
	maxIterations            uint32
	arithmeticPolicy         bool
	arithmeticPivotStrategy  uint32
	arithmeticBlandThreshold uint32
	arithmeticBland          bool
	infeasible               uint32
	Stats                    Stats
}

func NewSolver(limit *control.Reslimit) *Solver {
	return &Solver{limit: limit, patch: make(map[uint32]bool), maxIterations: ^uint32(0), infeasible: NullVariable}
}
func (s *Solver) EnsureVariable(v uint32) {
	for uint32(len(s.vars)) <= v {
		s.matrix.EnsureVariable(uint32(len(s.vars)))
		s.vars = append(s.vars, variableInfo{})
	}
}
func (s *Solver) NumVariables() uint32      { return uint32(len(s.vars)) }
func (s *Solver) Matrix() *Matrix           { return &s.matrix }
func (s *Solver) SetMaxIterations(n uint32) { s.maxIterations = n }

func (s *Solver) isBase(v uint32) bool  { return int(v) < len(s.vars) && s.vars[v].base }
func (s *Solver) outside(v uint32) bool { return s.belowLower(v) || s.aboveUpper(v) }
func (s *Solver) belowLower(v uint32) bool {
	x := &s.vars[v]
	return x.lowerValid && x.value.Cmp(&x.lower) < 0
}
func (s *Solver) aboveUpper(v uint32) bool {
	x := &s.vars[v]
	return x.upperValid && x.value.Cmp(&x.upper) > 0
}
func (s *Solver) aboveLower(v uint32) bool {
	x := &s.vars[v]
	return !x.lowerValid || x.value.Cmp(&x.lower) > 0
}
func (s *Solver) belowUpper(v uint32) bool {
	x := &s.vars[v]
	return !x.upperValid || x.value.Cmp(&x.upper) < 0
}
func (s *Solver) addPatch(v uint32) {
	if s.isBase(v) && s.outside(v) {
		s.patch[v] = true
	}
}

func (s *Solver) AddRationalRow(base uint32, variables []uint32, coefficients []big.Rat) (Row, error) {
	if len(variables) != len(coefficients) {
		return Row{}, fmt.Errorf("simplex: row length mismatch")
	}
	s.EnsureVariable(base)
	if s.isBase(base) {
		return Row{}, fmt.Errorf("simplex: base variable %d is already basic", base)
	}
	r := s.matrix.NewRow()
	for i, v := range variables {
		s.EnsureVariable(v)
		if coefficients[i].Sign() != 0 {
			if err := s.matrix.AddRational(r, &coefficients[i], v); err != nil {
				_ = s.matrix.Delete(r)
				return Row{}, err
			}
		}
	}
	// Eliminate existing base variables. A rational multiple is algebraically
	// equivalent to the source LCM scaling and avoids a template-specific path.
	for {
		entries, _ := s.matrix.Entries(r)
		changed := false
		for _, e := range entries {
			if !s.isBase(e.Variable) {
				continue
			}
			b := &s.vars[e.Variable]
			factor := new(big.Rat).Neg(new(big.Rat).Quo(e.Coefficient, &b.baseCoeff))
			if err := s.matrix.Add(r, factor, Row{b.row}); err != nil {
				_ = s.matrix.Delete(r)
				return Row{}, err
			}
			changed = true
			break
		}
		if !changed {
			break
		}
	}
	baseCoeff, ok := s.matrix.coefficient(r, base)
	if !ok || baseCoeff.Sign() == 0 {
		_ = s.matrix.Delete(r)
		return Row{}, fmt.Errorf("simplex: base variable has zero coefficient")
	}
	var value Infinitesimal
	entries, _ := s.matrix.Entries(r)
	for _, e := range entries {
		if e.Variable != base {
			term := s.vars[e.Variable].value.Clone()
			term.Mul(e.Coefficient)
			value.Add(&term)
		}
	}
	value.Neg()
	value.Quo(baseCoeff)
	for len(s.rowBase) <= int(r.id) {
		s.rowBase = append(s.rowBase, NullVariable)
	}
	s.rowBase[r.id] = base
	v := &s.vars[base]
	v.row = r.id
	v.base = true
	v.baseCoeff.Set(baseCoeff)
	v.value.Set(&value)
	s.addPatch(base)
	return r, nil
}

func (s *Solver) AddIntegerRow(base uint32, variables []uint32, coefficients []big.Int) (Row, error) {
	if len(variables) != len(coefficients) {
		return Row{}, fmt.Errorf("simplex: row length mismatch")
	}
	q := make([]big.Rat, len(coefficients))
	for i := range q {
		q[i].SetInt(&coefficients[i])
	}
	return s.AddRationalRow(base, variables, q)
}

func (s *Solver) updateValueCore(v uint32, delta *Infinitesimal) {
	s.vars[v].value.Add(delta)
	s.addPatch(v)
}
func (s *Solver) updateValue(v uint32, delta *Infinitesimal) {
	if delta.IsZero() {
		return
	}
	s.updateValueCore(v, delta)
	for _, r := range s.matrix.ColumnRows(v) {
		base := s.rowBase[r.id]
		// Source update_value also visits a basic variable's own row.
		// Its opposite contribution cancels the initial delta, preserving
		// the canonical equation when a caller seeds a basic variable.
		coeff, _ := s.matrix.coefficient(r, v)
		d := delta.Clone()
		d.Mul(coeff)
		d.Quo(&s.vars[base].baseCoeff)
		d.Neg()
		s.updateValueCore(base, &d)
	}
}

func (s *Solver) SetLower(v uint32, bound Infinitesimal) error {
	s.boundConflict, s.boundConflictKnown = nil, false
	s.EnsureVariable(v)
	x := &s.vars[v]
	if x.upperValid && bound.Cmp(&x.upper) > 0 {
		return fmt.Errorf("simplex: lower bound exceeds upper bound")
	}
	x.lower.Set(&bound)
	x.lowerValid = true
	x.lowerDependencies, x.lowerExplained = nil, false
	if !x.base && x.value.Cmp(&bound) < 0 {
		d := bound.Clone()
		d.Sub(&x.value)
		s.updateValue(v, &d)
	} else {
		s.addPatch(v)
	}
	return nil
}
func (s *Solver) SetUpper(v uint32, bound Infinitesimal) error {
	s.boundConflict, s.boundConflictKnown = nil, false
	s.EnsureVariable(v)
	x := &s.vars[v]
	if x.lowerValid && x.lower.Cmp(&bound) > 0 {
		return fmt.Errorf("simplex: upper bound is below lower bound")
	}
	x.upper.Set(&bound)
	x.upperValid = true
	x.upperDependencies, x.upperExplained = nil, false
	if !x.base && x.value.Cmp(&bound) > 0 {
		d := bound.Clone()
		d.Sub(&x.value)
		s.updateValue(v, &d)
	} else {
		s.addPatch(v)
	}
	return nil
}
func (s *Solver) UnsetLower(v uint32) {
	delete(s.arithmeticBoundAtoms, arithmeticBoundKey{v, true})
	s.boundConflict, s.boundConflictKnown = nil, false
	if int(v) < len(s.vars) {
		s.vars[v].lowerValid = false
		s.vars[v].lowerDependencies, s.vars[v].lowerExplained = nil, false
	}
}
func (s *Solver) UnsetUpper(v uint32) {
	delete(s.arithmeticBoundAtoms, arithmeticBoundKey{v, false})
	s.boundConflict, s.boundConflictKnown = nil, false
	if int(v) < len(s.vars) {
		s.vars[v].upperValid = false
		s.vars[v].upperDependencies, s.vars[v].upperExplained = nil, false
	}
}
func (s *Solver) Lower(v uint32) (Infinitesimal, bool) {
	if int(v) >= len(s.vars) {
		return Infinitesimal{}, false
	}
	x := &s.vars[v]
	return x.lower.Clone(), x.lowerValid
}
func (s *Solver) Upper(v uint32) (Infinitesimal, bool) {
	if int(v) >= len(s.vars) {
		return Infinitesimal{}, false
	}
	x := &s.vars[v]
	return x.upper.Clone(), x.upperValid
}
func (s *Solver) Value(v uint32) Infinitesimal {
	if int(v) >= len(s.vars) {
		return Infinitesimal{}
	}
	return s.vars[v].value.Clone()
}
func (s *Solver) SetValue(v uint32, value Infinitesimal) {
	s.EnsureVariable(v)
	d := value.Clone()
	d.Sub(&s.vars[v].value)
	s.updateValue(v, &d)
}

func (s *Solver) selectPatch() uint32 {
	keys := make([]uint32, 0, len(s.patch))
	for v := range s.patch {
		keys = append(keys, v)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, v := range keys {
		delete(s.patch, v)
		if s.outside(v) {
			return v
		}
	}
	return NullVariable
}
func (s *Solver) nonFreeDependencies(v uint32) int {
	n := 0
	x := &s.vars[v]
	if x.lowerValid || x.upperValid {
		n++
	}
	for _, r := range s.matrix.ColumnRows(v) {
		b := s.rowBase[r.id]
		if b == v {
			continue
		}
		x = &s.vars[b]
		if x.lowerValid || x.upperValid {
			n++
		}
	}
	return n
}
func (s *Solver) selectPivot(base uint32, below bool) (uint32, *big.Rat) {
	entries, _ := s.matrix.Entries(Row{s.vars[base].row})
	result := NullVariable
	bestDep := int(^uint(0) >> 1)
	bestCol := ^uint32(0)
	for _, e := range entries {
		v := e.Variable
		if v == base {
			continue
		}
		negative := e.Coefficient.Sign() < 0
		wrongDirection := negative
		if !below {
			wrongDirection = e.Coefficient.Sign() > 0
		}
		can := (!wrongDirection && s.aboveLower(v)) || (wrongDirection && s.belowUpper(v))
		if !can {
			continue
		}
		if s.arithmeticBland {
			if v < result {
				result = v
			}
			continue
		}
		dep, col := s.nonFreeDependencies(v), s.matrix.ColumnSize(v)
		if dep < bestDep || (dep == bestDep && (col < bestCol || (col == bestCol && v < result))) {
			result = v
			bestDep = dep
			bestCol = col
		}
	}
	if result == NullVariable {
		return result, nil
	}
	c, _ := s.matrix.coefficient(Row{s.vars[base].row}, result)
	return result, new(big.Rat).Set(c)
}

func (s *Solver) pivot(oldBase, newBase uint32, pivotCoeff *big.Rat) error {
	r := Row{s.vars[oldBase].row}
	rows := s.matrix.ColumnRows(newBase)
	s.rowBase[r.id] = newBase
	n := &s.vars[newBase]
	n.row = r.id
	n.base = true
	n.baseCoeff.Set(pivotCoeff)
	s.vars[oldBase].base = false
	for _, other := range rows {
		if other == r {
			continue
		}
		akj, ok := s.matrix.coefficient(other, newBase)
		if !ok {
			continue
		}
		factor := new(big.Rat).Neg(new(big.Rat).Quo(new(big.Rat).Set(akj), pivotCoeff))
		if err := s.matrix.Add(other, factor, r); err != nil {
			return err
		}
		b := s.rowBase[other.id]
		coeff, ok := s.matrix.coefficient(other, b)
		if !ok {
			return fmt.Errorf("simplex: pivot lost base coefficient")
		}
		s.vars[b].baseCoeff.Set(coeff)
	}
	s.Stats.NumPivots++
	s.addPatch(newBase)
	return nil
}
func (s *Solver) updateAndPivot(oldBase, newBase uint32, pivotCoeff *big.Rat, newValue *Infinitesimal) error {
	theta := s.vars[oldBase].value.Clone()
	theta.Sub(newValue)
	theta.Mul(&s.vars[oldBase].baseCoeff)
	theta.Quo(pivotCoeff)
	s.updateValue(newBase, &theta)
	return s.pivot(oldBase, newBase, pivotCoeff)
}

func (s *Solver) makeVariableFeasible(v uint32) (bool, error) {
	x := &s.vars[v]
	var target Infinitesimal
	below := false
	if s.belowLower(v) {
		target = x.lower.Clone()
		below = x.baseCoeff.Sign() > 0
	} else if s.aboveUpper(v) {
		target = x.upper.Clone()
		below = x.baseCoeff.Sign() < 0
	} else {
		return true, nil
	}
	pivot, coeff := s.selectPivot(v, below)
	if pivot == NullVariable {
		return false, nil
	}
	return true, s.updateAndPivot(v, pivot, coeff, &target)
}

func (s *Solver) MakeFeasible() (lbool.LBool, error) {
	s.boundConflict, s.boundConflictKnown = nil, false
	s.Stats.NumChecks++
	s.infeasible = NullVariable
	s.arithmeticBland = false
	var left map[uint32]bool
	if s.arithmeticPolicy {
		left = make(map[uint32]bool)
	}
	var repeated uint32
	for iteration := uint32(0); ; iteration++ {
		var v uint32
		if s.arithmeticPolicy {
			v = s.selectArithmeticPatch()
		} else {
			v = s.selectPatch()
		}
		if v == NullVariable {
			return lbool.True, nil
		}
		if (s.limit != nil && !s.limit.Inc()) || iteration > s.maxIterations {
			return lbool.Undef, nil
		}
		if s.arithmeticPolicy {
			s.checkArithmeticBlands(v, left, &repeated)
		}
		ok, err := s.makeVariableFeasible(v)
		if err != nil {
			return lbool.Undef, err
		}
		if !ok {
			s.patch[v] = true
			s.infeasible = v
			s.Stats.NumInfeasible++
			return lbool.False, nil
		}
	}
}

func (s *Solver) InfeasibleRow() (Row, error) {
	if s.infeasible == NullVariable || !s.isBase(s.infeasible) {
		return Row{}, fmt.Errorf("simplex: no infeasible row")
	}
	return Row{s.vars[s.infeasible].row}, nil
}
func (s *Solver) BaseVariable(r Row) (uint32, error) {
	if !s.matrix.valid(r) || int(r.id) >= len(s.rowBase) || s.rowBase[r.id] == NullVariable {
		return 0, fmt.Errorf("simplex: row has no base")
	}
	return s.rowBase[r.id], nil
}
func (s *Solver) BaseCoefficient(r Row) (*big.Rat, error) {
	v, e := s.BaseVariable(r)
	if e != nil {
		return nil, e
	}
	return new(big.Rat).Set(&s.vars[v].baseCoeff), nil
}
func (s *Solver) RowEntries(r Row) ([]RowEntry, error) { return s.matrix.Entries(r) }

// ColumnRows and BasicRow expose the concrete tableau incidence information
// needed by the retained arithmetic theory's nonlinear cluster traversal.
func (s *Solver) ColumnRows(variable uint32) []Row { return s.matrix.ColumnRows(variable) }

// BasicRows returns live rows in source row-vector order, including holes
// reused by later row creation.
func (s *Solver) BasicRows() []Row {
	var rows []Row
	for index, base := range s.rowBase {
		if base != NullVariable {
			rows = append(rows, Row{uint32(index)})
		}
	}
	return rows
}

func (s *Solver) BasicRow(variable uint32) (Row, bool) {
	if !s.isBase(variable) {
		return Row{}, false
	}
	return Row{s.vars[variable].row}, true
}

// DeleteRow removes the row containing variable. A nonbasic variable is first
// pivoted into its first row, preserving the source operation order.
func (s *Solver) DeleteRow(variable uint32) error {
	if int(variable) >= len(s.vars) {
		return nil
	}
	var r Row
	if s.isBase(variable) {
		r = Row{s.vars[variable].row}
	} else {
		rows := s.matrix.ColumnRows(variable)
		if len(rows) == 0 {
			return nil
		}
		r = rows[0]
		old := s.rowBase[r.id]
		target := s.vars[old].value.Clone()
		if s.belowLower(old) {
			target = s.vars[old].lower.Clone()
		} else if s.aboveUpper(old) {
			target = s.vars[old].upper.Clone()
		}
		coeff, _ := s.matrix.coefficient(r, variable)
		if err := s.updateAndPivot(old, variable, new(big.Rat).Set(coeff), &target); err != nil {
			return err
		}
	}
	base := s.rowBase[r.id]
	delete(s.patch, base)
	x := &s.vars[base]
	x.base, x.lowerValid, x.upperValid = false, false, false
	s.rowBase[r.id] = NullVariable
	return s.matrix.Delete(r)
}

func (s *Solver) atLower(v uint32) bool {
	x := &s.vars[v]
	return x.lowerValid && x.value.Cmp(&x.lower) == 0
}
func (s *Solver) atUpper(v uint32) bool {
	x := &s.vars[v]
	return x.upperValid && x.value.Cmp(&x.upper) == 0
}

func (s *Solver) pickLeaving(x uint32, increase bool) (uint32, Infinitesimal, *big.Rat, bool) {
	selected := NullVariable
	var gain Infinitesimal
	var selectedCoeff *big.Rat
	selectedIncrease := false
	for _, r := range s.matrix.ColumnRows(x) {
		base := s.rowBase[r.id]
		if base == x {
			continue
		}
		v := &s.vars[base]
		coeff, _ := s.matrix.coefficient(r, x)
		incBase := (v.baseCoeff.Sign() == coeff.Sign()) != increase
		if (incBase && !v.upperValid) || (!incBase && !v.lowerValid) {
			continue
		}
		current := v.value.Clone()
		if incBase {
			current.Sub(&v.upper)
		} else {
			current.Sub(&v.lower)
		}
		current.Mul(&v.baseCoeff)
		current.Quo(coeff)
		current.Abs()
		if selected == NullVariable || current.Cmp(&gain) < 0 || (current.IsZero() && gain.IsZero() && base < selected) {
			selected, gain, selectedCoeff, selectedIncrease = base, current, new(big.Rat).Set(coeff), incBase
		}
	}
	return selected, gain, selectedCoeff, selectedIncrease
}

func (s *Solver) selectPrimal(objective uint32) (leave, enter uint32, coeff *big.Rat, incLeave, incEnter bool) {
	leave, enter = NullVariable, NullVariable
	entries, _ := s.matrix.Entries(Row{s.vars[objective].row})
	var best Infinitesimal
	for _, e := range entries {
		x := e.Variable
		if x == objective {
			continue
		}
		increase := e.Coefficient.Sign() == s.vars[objective].baseCoeff.Sign()
		if (increase && s.atUpper(x)) || (!increase && s.atLower(x)) {
			continue
		}
		candidate, gain, a, inc := s.pickLeaving(x, increase)
		if candidate == NullVariable {
			return candidate, x, a, inc, increase
		}
		if leave == NullVariable || gain.Cmp(&best) > 0 || (gain.IsZero() && best.IsZero() && candidate < leave) {
			leave, enter, coeff, incLeave, incEnter, best = candidate, x, a, inc, increase, gain
		}
	}
	return
}

func (s *Solver) moveToBound(v uint32, toLower bool) {
	x := &s.vars[v]
	var delta Infinitesimal
	if toLower {
		delta = x.value.Clone()
		delta.Sub(&x.lower)
	} else {
		delta = x.upper.Clone()
		delta.Sub(&x.value)
	}
	for _, r := range s.matrix.ColumnRows(v) {
		base := s.rowBase[r.id]
		if base == v {
			continue
		}
		b := &s.vars[base]
		coeff, _ := s.matrix.coefficient(r, v)
		baseToLower := ((coeff.Sign() > 0) != (b.baseCoeff.Sign() > 0)) == toLower
		var bound *Infinitesimal
		if !baseToLower && b.upperValid {
			bound = &b.upper
		} else if baseToLower && b.lowerValid {
			bound = &b.lower
		}
		if bound != nil {
			d := bound.Clone()
			d.Sub(&b.value)
			d.Mul(&b.baseCoeff)
			d.Quo(coeff)
			d.Abs()
			if d.Cmp(&delta) < 0 {
				delta = d
			}
		}
	}
	if toLower {
		delta.Neg()
	}
	s.updateValue(v, &delta)
}

// Minimize returns True for an optimum, False for an unbounded objective and
// Undef for a resource stop.
func (s *Solver) Minimize(objective uint32) (lbool.LBool, error) {
	if !s.isBase(objective) {
		return lbool.Undef, fmt.Errorf("simplex: objective is not basic")
	}
	for {
		if s.limit != nil && !s.limit.Inc() {
			return lbool.Undef, nil
		}
		leave, enter, coeff, incLeave, incEnter := s.selectPrimal(objective)
		if enter == NullVariable {
			return lbool.True, nil
		}
		if leave == NullVariable {
			x := &s.vars[enter]
			var d Infinitesimal
			if incEnter && x.upperValid {
				d = x.upper.Clone()
				d.Sub(&x.value)
			} else if !incEnter && x.lowerValid {
				d = x.lower.Clone()
				d.Sub(&x.value)
			} else {
				return lbool.False, nil
			}
			s.updateValue(enter, &d)
			continue
		}
		if coeff == nil {
			return lbool.Undef, fmt.Errorf("simplex: missing pivot coefficient")
		}
		if err := s.pivot(leave, enter, coeff); err != nil {
			return lbool.Undef, err
		}
		s.moveToBound(leave, !incLeave)
	}
}

func (s *Solver) Reset() {
	s.arithmeticBoundAtoms = nil
	s.boundConflict, s.boundConflictKnown = nil, false
	s.matrix.Reset()
	s.vars = nil
	s.rowBase = nil
	s.patch = make(map[uint32]bool)
	s.infeasible = NullVariable
}
func (s *Solver) Display(w io.Writer) error {
	if err := s.matrix.Display(w); err != nil {
		return err
	}
	for i := range s.vars {
		x := &s.vars[i]
		lo, hi := "-oo", "oo"
		if x.lowerValid {
			lo = x.lower.String()
		}
		if x.upperValid {
			hi = x.upper.String()
		}
		if _, err := fmt.Fprintf(w, "v%d %s [%s:%s]", i, x.value.String(), lo, hi); err != nil {
			return err
		}
		if x.base {
			if _, err := fmt.Fprintf(w, " b:%d", x.row); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}
