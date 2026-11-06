package simplex

import (
	"fmt"
	"io"
	"math/big"
	"sort"
)

type Relation uint8

const (
	Equal Relation = iota
	Less
	LessEqual
	Divides
)

func (r Relation) String() string {
	switch r {
	case Equal:
		return "="
	case Less:
		return "<"
	case LessEqual:
		return "<="
	case Divides:
		return "mod"
	default:
		return "?"
	}
}

type LinearTerm struct {
	Variable    uint32
	Coefficient big.Rat
}

func NewLinearTerm(variable uint32, coefficient *big.Rat) LinearTerm {
	var t LinearTerm
	t.Variable = variable
	if coefficient != nil {
		t.Coefficient.Set(coefficient)
	}
	return t
}

type OptRow struct {
	Terms                    []LinearTerm
	Constant, Modulus, Value big.Rat
	Relation                 Relation
	Alive                    bool
}

func (r *OptRow) Clone() OptRow {
	var x OptRow
	x.Terms = cloneTerms(r.Terms)
	x.Constant.Set(&r.Constant)
	x.Modulus.Set(&r.Modulus)
	x.Value.Set(&r.Value)
	x.Relation = r.Relation
	x.Alive = r.Alive
	return x
}
func cloneTerms(v []LinearTerm) []LinearTerm {
	r := make([]LinearTerm, len(v))
	for i := range v {
		r[i] = NewLinearTerm(v[i].Variable, &v[i].Coefficient)
	}
	return r
}

type Optimum struct {
	Infinite       bool
	Value, Epsilon big.Rat
}

type ModelBasedOpt struct {
	rows      []OptRow
	values    []big.Rat
	integers  []bool
	incidence [][]uint32
	retired   []uint32
}

func NewModelBasedOpt() *ModelBasedOpt { return &ModelBasedOpt{rows: []OptRow{{}}} }
func (m *ModelBasedOpt) AddVariable(value *big.Rat, integer bool) uint32 {
	var q big.Rat
	if value != nil {
		q.Set(value)
	}
	id := uint32(len(m.values))
	m.values = append(m.values, q)
	m.integers = append(m.integers, integer)
	m.incidence = append(m.incidence, nil)
	return id
}
func (m *ModelBasedOpt) Value(v uint32) (*big.Rat, error) {
	if int(v) >= len(m.values) {
		return nil, fmt.Errorf("simplex: invalid model variable")
	}
	return new(big.Rat).Set(&m.values[v]), nil
}
func (m *ModelBasedOpt) rowValue(r *OptRow) *big.Rat {
	v := new(big.Rat).Set(&r.Constant)
	for i := range r.Terms {
		v.Add(v, new(big.Rat).Mul(&r.Terms[i].Coefficient, &m.values[r.Terms[i].Variable]))
	}
	return v
}
func (m *ModelBasedOpt) canonical(terms []LinearTerm) ([]LinearTerm, error) {
	values := make(map[uint32]*big.Rat)
	var order []uint32
	for _, t := range terms {
		if int(t.Variable) >= len(m.values) {
			return nil, fmt.Errorf("simplex: invalid model variable")
		}
		if _, ok := values[t.Variable]; !ok {
			values[t.Variable] = new(big.Rat)
			order = append(order, t.Variable)
		}
		values[t.Variable].Add(values[t.Variable], &t.Coefficient)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	out := make([]LinearTerm, 0, len(order))
	for _, v := range order {
		if values[v].Sign() != 0 {
			out = append(out, NewLinearTerm(v, values[v]))
		}
	}
	return out, nil
}
func (m *ModelBasedOpt) newRow() uint32 {
	if len(m.retired) > 0 {
		id := m.retired[len(m.retired)-1]
		m.retired = m.retired[:len(m.retired)-1]
		m.rows[id] = OptRow{}
		return id
	}
	id := uint32(len(m.rows))
	m.rows = append(m.rows, OptRow{})
	return id
}
func (m *ModelBasedOpt) setRow(id uint32, terms []LinearTerm, constant, modulus *big.Rat, relation Relation) error {
	canonical, err := m.canonical(terms)
	if err != nil {
		return err
	}
	r := &m.rows[id]
	*r = OptRow{}
	r.Terms = canonical
	r.Relation = relation
	r.Alive = true
	if constant != nil {
		r.Constant.Set(constant)
	}
	if modulus != nil {
		r.Modulus.Set(modulus)
	}
	integerRow := true
	for _, t := range canonical {
		integerRow = integerRow && m.integers[t.Variable]
		if m.integers[t.Variable] && !t.Coefficient.IsInt() {
			return fmt.Errorf("simplex: nonintegral integer coefficient")
		}
	}
	r.Value.Set(m.rowValue(r))
	if integerRow && relation == Less {
		r.Relation = LessEqual
		r.Constant.Add(&r.Constant, big.NewRat(1, 1))
		r.Value.Add(&r.Value, big.NewRat(1, 1))
	}
	return nil
}
func (m *ModelBasedOpt) AddConstraint(terms []LinearTerm, constant *big.Rat, relation Relation) error {
	return m.addConstraint(terms, constant, nil, relation)
}
func (m *ModelBasedOpt) AddDivides(terms []LinearTerm, constant, modulus *big.Rat) error {
	return m.addConstraint(terms, constant, modulus, Divides)
}
func (m *ModelBasedOpt) addConstraint(terms []LinearTerm, constant, modulus *big.Rat, relation Relation) error {
	id := m.newRow()
	if err := m.setRow(id, terms, constant, modulus, relation); err != nil {
		m.retire(id)
		return err
	}
	for _, t := range m.rows[id].Terms {
		m.incidence[t.Variable] = append(m.incidence[t.Variable], id)
	}
	if err := m.checkRow(id); err != nil {
		m.retire(id)
		return err
	}
	return nil
}
func (m *ModelBasedOpt) SetObjective(terms []LinearTerm, constant *big.Rat) error {
	return m.setRow(0, terms, constant, nil, LessEqual)
}
func (m *ModelBasedOpt) coefficient(row uint32, v uint32) *big.Rat {
	r := &m.rows[row]
	i := sort.Search(len(r.Terms), func(i int) bool { return r.Terms[i].Variable >= v })
	if i < len(r.Terms) && r.Terms[i].Variable == v {
		return new(big.Rat).Set(&r.Terms[i].Coefficient)
	}
	return new(big.Rat)
}
func (m *ModelBasedOpt) checkRow(id uint32) error {
	r := &m.rows[id]
	if id == 0 {
		return nil
	}
	switch r.Relation {
	case Equal:
		if r.Value.Sign() != 0 {
			return fmt.Errorf("simplex: equality not satisfied by model")
		}
	case Less:
		if r.Value.Sign() >= 0 {
			return fmt.Errorf("simplex: strict inequality not satisfied by model")
		}
	case LessEqual:
		if r.Value.Sign() > 0 {
			return fmt.Errorf("simplex: inequality not satisfied by model")
		}
	case Divides:
		if r.Modulus.Sign() == 0 || ratMod(&r.Value, &r.Modulus).Sign() != 0 {
			return fmt.Errorf("simplex: divisibility not satisfied by model")
		}
	}
	return nil
}
func ratMod(a, b *big.Rat) *big.Rat {
	if !a.IsInt() || !b.IsInt() || b.Sign() == 0 {
		return new(big.Rat).Set(a)
	}
	x := new(big.Int).Mod(a.Num(), new(big.Int).Abs(b.Num()))
	return new(big.Rat).SetInt(x)
}
func (m *ModelBasedOpt) retire(id uint32) {
	if id == 0 || !m.rows[id].Alive {
		return
	}
	m.rows[id].Alive = false
	m.retired = append(m.retired, id)
}

func (m *ModelBasedOpt) combine(dst uint32, c *big.Rat, src uint32, sameSign bool) error {
	a := &m.rows[dst]
	b := &m.rows[src]
	terms := append(cloneTerms(a.Terms), func() []LinearTerm {
		x := cloneTerms(b.Terms)
		for i := range x {
			x[i].Coefficient.Mul(&x[i].Coefficient, c)
		}
		return x
	}()...)
	canonical, err := m.canonical(terms)
	if err != nil {
		return err
	}
	a.Terms = canonical
	a.Constant.Add(&a.Constant, new(big.Rat).Mul(c, &b.Constant))
	a.Value.Add(&a.Value, new(big.Rat).Mul(c, &b.Value))
	if !sameSign && b.Relation == Less {
		a.Relation = Less
	} else if sameSign && a.Relation == Less && b.Relation == Less {
		a.Relation = LessEqual
	}
	if dst != 0 {
		for _, t := range b.Terms {
			m.incidence[t.Variable] = append(m.incidence[t.Variable], dst)
		}
	}
	return m.checkRow(dst)
}
func (m *ModelBasedOpt) resolve(source uint32, a1 *big.Rat, destination, x uint32) error {
	if !m.rows[destination].Alive {
		return nil
	}
	a2 := m.coefficient(destination, x)
	if a2.Sign() == 0 {
		return nil
	}
	if m.integers[x] && m.rows[source].Relation == Equal {
		// Equality substitution must retain integral rows. Dividing by a1
		// produced fractional coefficients that cannot be reconstructed as
		// integer ASTs. Scale the destination and eliminate exactly instead;
		// inequality-only slack cuts do not apply to an equality source.
		factor := absRat(a1)
		m.scale(destination, factor)
		if m.rows[destination].Relation == Divides {
			m.rows[destination].Modulus.Mul(&m.rows[destination].Modulus, factor)
		}
		multiple := new(big.Rat).Set(a2)
		if a1.Sign() > 0 {
			multiple.Neg(multiple)
		}
		if err := m.combine(destination, multiple, source, false); err != nil {
			return err
		}
		m.normalize(destination)
		return nil
	}
	if m.integers[x] && m.rows[source].Relation != Equal {
		return m.resolveInteger(source, a1, destination, a2, x)
	}
	same := destination != 0 && a1.Sign() == a2.Sign()
	return m.combine(destination, new(big.Rat).Neg(new(big.Rat).Quo(a2, a1)), source, same)
}

func (m *ModelBasedOpt) scale(row uint32, c *big.Rat) {
	r := &m.rows[row]
	for i := range r.Terms {
		r.Terms[i].Coefficient.Mul(&r.Terms[i].Coefficient, c)
	}
	r.Constant.Mul(&r.Constant, c)
	r.Value.Mul(&r.Value, c)
}

func (m *ModelBasedOpt) addConstant(row uint32, c *big.Rat) {
	r := &m.rows[row]
	r.Constant.Add(&r.Constant, c)
	r.Value.Add(&r.Value, c)
}

func absRat(q *big.Rat) *big.Rat {
	return new(big.Rat).Abs(q)
}

func (m *ModelBasedOpt) normalize(row uint32) {
	r := &m.rows[row]
	if len(r.Terms) == 0 || r.Relation == Divides {
		return
	}
	g := new(big.Int).Abs(r.Terms[0].Coefficient.Num())
	if !r.Terms[0].Coefficient.IsInt() {
		return
	}
	for i := 1; i < len(r.Terms) && g.Cmp(big.NewInt(1)) != 0; i++ {
		if !r.Terms[i].Coefficient.IsInt() {
			return
		}
		g.GCD(nil, nil, g, new(big.Int).Abs(r.Terms[i].Coefficient.Num()))
	}
	if r.Constant.Sign() != 0 {
		if !r.Constant.IsInt() {
			return
		}
		g.GCD(nil, nil, g, new(big.Int).Abs(r.Constant.Num()))
	}
	if g.Sign() != 0 && g.Cmp(big.NewInt(1)) != 0 {
		m.scale(row, new(big.Rat).SetFrac(big.NewInt(1), g))
	}
}

func termsWithout(terms []LinearTerm, x uint32) []LinearTerm {
	out := make([]LinearTerm, 0, len(terms))
	for i := range terms {
		if terms[i].Variable != x {
			out = append(out, NewLinearTerm(terms[i].Variable, &terms[i].Coefficient))
		}
	}
	return out
}

func normalSign(q *big.Rat) *big.Rat {
	if q.Sign() > 0 {
		return big.NewRat(-1, 1)
	}
	return big.NewRat(1, 1)
}

func (m *ModelBasedOpt) resolveInteger(source uint32, sourceCoeff *big.Rat, destination uint32, destinationCoeff *big.Rat, x uint32) error {
	if !sourceCoeff.IsInt() || !destinationCoeff.IsInt() {
		return fmt.Errorf("simplex: integer resolution requires integral coefficients")
	}
	if sourceCoeff.Sign() == destinationCoeff.Sign() {
		m.scale(destination, absRat(sourceCoeff))
		if err := m.combine(destination, new(big.Rat).Neg(absRat(destinationCoeff)), source, false); err != nil {
			return err
		}
		m.normalize(destination)
		return nil
	}

	absSource := absRat(sourceCoeff)
	absDestination := absRat(destinationCoeff)
	slack := new(big.Rat).Mul(new(big.Rat).Sub(absSource, big.NewRat(1, 1)), new(big.Rat).Sub(absDestination, big.NewRat(1, 1)))
	xValue := &m.values[x]
	destinationValue := new(big.Rat).Sub(&m.rows[destination].Value, new(big.Rat).Mul(xValue, destinationCoeff))
	sourceValue := new(big.Rat).Sub(&m.rows[source].Value, new(big.Rat).Mul(xValue, sourceCoeff))
	caseValue := new(big.Rat).Add(new(big.Rat).Mul(sourceCoeff, destinationValue), new(big.Rat).Mul(destinationCoeff, sourceValue))
	caseValue.Add(caseValue, slack)
	if caseValue.Sign() <= 0 || absSource.Cmp(big.NewRat(1, 1)) == 0 || absDestination.Cmp(big.NewRat(1, 1)) == 0 {
		m.scale(destination, absSource)
		m.addConstant(destination, new(big.Rat).Neg(slack))
		return m.combine(destination, absDestination, source, false)
	}

	if absDestination.Cmp(absSource) <= 0 {
		z := ratMod(destinationValue, absDestination)
		if z.Sign() != 0 {
			z.Sub(absDestination, z)
		}
		if err := m.AddDivides(termsWithout(m.rows[destination].Terms, x), new(big.Rat).Add(&m.rows[destination].Constant, z), absDestination); err != nil {
			return err
		}
		m.addConstant(destination, z)
		m.scale(destination, new(big.Rat).Mul(sourceCoeff, normalSign(destinationCoeff)))
		return m.combine(destination, absDestination, source, false)
	}

	z := ratMod(sourceValue, absSource)
	if z.Sign() != 0 {
		z.Sub(absSource, z)
	}
	if err := m.AddDivides(termsWithout(m.rows[source].Terms, x), new(big.Rat).Add(&m.rows[source].Constant, z), absSource); err != nil {
		return err
	}
	m.scale(destination, absSource)
	m.addConstant(destination, new(big.Rat).Mul(z, new(big.Rat).Mul(destinationCoeff, normalSign(sourceCoeff))))
	return m.combine(destination, new(big.Rat).Mul(destinationCoeff, normalSign(sourceCoeff)), source, false)
}

func (m *ModelBasedOpt) findBound(x uint32, positive bool) (uint32, *big.Rat, []uint32, []uint32, bool) {
	selected := NullVariable
	var selectedValue *big.Rat
	var selectedCoeff *big.Rat
	var above, below []uint32
	seen := make(map[uint32]bool)
	xv := &m.values[x]
	for _, id := range m.incidence[x] {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		r := &m.rows[id]
		if !r.Alive {
			continue
		}
		a := m.coefficient(id, x)
		if a.Sign() == 0 {
			continue
		}
		if (a.Sign() > 0) == positive || r.Relation == Equal {
			value := new(big.Rat).Sub(xv, new(big.Rat).Quo(&r.Value, a))
			better := selected == NullVariable || (value.Cmp(selectedValue) == 0 && r.Relation == Less) || (positive && value.Cmp(selectedValue) < 0) || (!positive && value.Cmp(selectedValue) > 0)
			if better {
				if selected != NullVariable {
					above = append(above, selected)
				}
				selected, selectedValue, selectedCoeff = id, value, a
			} else {
				above = append(above, id)
			}
		} else {
			below = append(below, id)
		}
	}
	return selected, selectedCoeff, above, below, selected != NullVariable
}
func (m *ModelBasedOpt) updateValue(x uint32, value *big.Rat) {
	old := new(big.Rat).Set(&m.values[x])
	m.values[x].Set(value)
	seen := make(map[uint32]bool)
	for _, id := range m.incidence[x] {
		if seen[id] {
			continue
		}
		seen[id] = true
		a := m.coefficient(id, x)
		delta := new(big.Rat).Mul(a, new(big.Rat).Sub(value, old))
		m.rows[id].Value.Add(&m.rows[id].Value, delta)
	}
}

func (m *ModelBasedOpt) Maximize() (Optimum, error) {
	var trail, variables []uint32
	for len(m.rows[0].Terms) > 0 {
		v := m.rows[0].Terms[len(m.rows[0].Terms)-1]
		bound, a, above, below, ok := m.findBound(v.Variable, v.Coefficient.Sign() > 0)
		if !ok {
			m.restoreBounds(variables, trail)
			return Optimum{Infinite: true}, nil
		}
		for _, id := range above {
			if err := m.resolve(bound, a, id, v.Variable); err != nil {
				return Optimum{}, err
			}
		}
		for _, id := range below {
			if err := m.resolve(bound, a, id, v.Variable); err != nil {
				return Optimum{}, err
			}
		}
		factor := new(big.Rat).Neg(new(big.Rat).Quo(&v.Coefficient, a))
		if err := m.combine(0, factor, bound, false); err != nil {
			return Optimum{}, err
		}
		m.retire(bound)
		trail = append(trail, bound)
		variables = append(variables, v.Variable)
	}
	m.restoreBounds(variables, trail)
	var result Optimum
	result.Value.Set(&m.rows[0].Value)
	if m.rows[0].Relation == Less {
		result.Epsilon.SetInt64(-1)
	}
	return result, nil
}
func (m *ModelBasedOpt) restoreBounds(vars, trail []uint32) {
	for i := len(trail) - 1; i >= 0; i-- {
		x, rid := vars[i], trail[i]
		r := &m.rows[rid]
		sum := new(big.Rat).Set(&r.Constant)
		a := new(big.Rat)
		for _, t := range r.Terms {
			if t.Variable == x {
				a.Set(&t.Coefficient)
			} else {
				sum.Add(sum, new(big.Rat).Mul(&t.Coefficient, &m.values[t.Variable]))
			}
		}
		next := new(big.Rat).Neg(new(big.Rat).Quo(sum, a))
		if r.Relation == Less {
			distance := new(big.Rat).Sub(&m.values[x], next)
			if distance.Sign() < 0 {
				distance.Neg(distance)
			}
			distance.Quo(distance, big.NewRat(2, 1))
			if distance.Sign() == 0 || distance.Cmp(big.NewRat(1, 1)) > 0 {
				distance.SetInt64(1)
			}
			if a.Sign() > 0 {
				next.Sub(next, distance)
			} else {
				next.Add(next, distance)
			}
		}
		m.values[x].Set(next)
	}
	for i := range m.rows {
		m.rows[i].Value.Set(m.rowValue(&m.rows[i]))
	}
}

func (m *ModelBasedOpt) copyRow(source uint32) (uint32, error) {
	id := m.newRow()
	r := &m.rows[source]
	if err := m.setRow(id, r.Terms, &r.Constant, &r.Modulus, r.Relation); err != nil {
		m.retire(id)
		return 0, err
	}
	for _, term := range m.rows[id].Terms {
		m.incidence[term.Variable] = append(m.incidence[term.Variable], id)
	}
	return id, nil
}

func (m *ModelBasedOpt) replaceWithConstant(row, x uint32, constant *big.Rat) {
	r := &m.rows[row]
	coefficient := m.coefficient(row, x)
	if coefficient.Sign() == 0 {
		return
	}
	next := r.Terms[:0]
	for i := range r.Terms {
		if r.Terms[i].Variable != x {
			next = append(next, r.Terms[i])
		}
	}
	r.Terms = next
	r.Constant.Add(&r.Constant, new(big.Rat).Mul(coefficient, constant))
	r.Value.Add(&r.Value, new(big.Rat).Mul(coefficient, new(big.Rat).Sub(constant, &m.values[x])))
}

func (m *ModelBasedOpt) replaceWithAffine(row, x uint32, scale *big.Rat, y uint32, constant *big.Rat) error {
	if !m.rows[row].Alive {
		return nil
	}
	coefficient := m.coefficient(row, x)
	if coefficient.Sign() == 0 {
		return nil
	}
	m.replaceWithConstant(row, x, constant)
	terms := append(cloneTerms(m.rows[row].Terms), NewLinearTerm(y, new(big.Rat).Mul(coefficient, scale)))
	canonical, err := m.canonical(terms)
	if err != nil {
		return err
	}
	m.rows[row].Terms = canonical
	m.rows[row].Value.Add(&m.rows[row].Value, new(big.Rat).Mul(coefficient, new(big.Rat).Mul(scale, &m.values[y])))
	m.incidence[y] = append(m.incidence[y], row)
	return m.checkRow(row)
}

func (m *ModelBasedOpt) solveFor(row, x uint32) error {
	coefficient := m.coefficient(row, x)
	if coefficient.Sign() == 0 || m.rows[row].Relation != Equal {
		return fmt.Errorf("simplex: invalid equality projection")
	}
	if m.integers[x] && absRat(coefficient).Cmp(big.NewRat(1, 1)) != 0 {
		if err := m.AddDivides(termsWithout(m.rows[row].Terms, x), &m.rows[row].Constant, absRat(coefficient)); err != nil {
			return err
		}
	}
	seen := map[uint32]bool{row: true}
	for _, other := range m.incidence[x] {
		if seen[other] {
			continue
		}
		seen[other] = true
		if m.rows[other].Alive && m.coefficient(other, x).Sign() != 0 {
			if err := m.resolve(row, coefficient, other, x); err != nil {
				return err
			}
		}
	}
	m.retire(row)
	return nil
}

func lcmRat(a, b *big.Rat) (*big.Rat, error) {
	if !a.IsInt() || !b.IsInt() {
		return nil, fmt.Errorf("simplex: divisibility modulus must be integral")
	}
	aa := new(big.Int).Abs(a.Num())
	bb := new(big.Int).Abs(b.Num())
	if aa.Sign() == 0 || bb.Sign() == 0 {
		return new(big.Rat), nil
	}
	g := new(big.Int).GCD(nil, nil, aa, bb)
	lcm := new(big.Int).Quo(new(big.Int).Mul(aa, bb), g)
	return new(big.Rat).SetInt(lcm), nil
}

func (m *ModelBasedOpt) solveMod(x uint32, rows []uint32) error {
	divisor := big.NewRat(1, 1)
	for _, row := range rows {
		var err error
		divisor, err = lcmRat(divisor, &m.rows[row].Modulus)
		if err != nil {
			return err
		}
	}
	if divisor.Sign() == 0 {
		return fmt.Errorf("simplex: modulo zero is not defined")
	}
	residue := ratMod(&m.values[x], divisor)
	for _, row := range rows {
		m.replaceWithConstant(row, x, residue)
		if err := m.checkRow(row); err != nil {
			return err
		}
	}
	newValue := new(big.Rat).Quo(new(big.Rat).Sub(&m.values[x], residue), divisor)
	if !newValue.IsInt() {
		return fmt.Errorf("simplex: modular projection produced a nonintegral quotient")
	}
	y := m.AddVariable(newValue, true)
	seen := make(map[uint32]bool)
	for _, row := range m.incidence[x] {
		if seen[row] {
			continue
		}
		seen[row] = true
		if err := m.replaceWithAffine(row, x, divisor, y, residue); err != nil {
			return err
		}
	}
	return m.projectOne(y)
}

// Project eliminates the variables in order while preserving the rows selected
// by the current model, matching the source model-based projection algorithm.
func (m *ModelBasedOpt) Project(variables ...uint32) error {
	for _, variable := range variables {
		if int(variable) >= len(m.values) {
			return fmt.Errorf("simplex: invalid model variable")
		}
		if err := m.projectOne(variable); err != nil {
			return err
		}
	}
	return nil
}

func (m *ModelBasedOpt) projectOne(x uint32) error {
	var upper, lower, modular []uint32
	upperIndex, lowerIndex := NullVariable, NullVariable
	var upperValue, lowerValue *big.Rat
	upperStrict, lowerStrict := false, false
	// The retained source initializes both flags false and updates them with
	// &=, leaving the small cross-product shortcut disabled for integers.
	upperUnit, lowerUnit := false, false
	seen := make(map[uint32]bool)
	for _, row := range m.incidence[x] {
		if seen[row] {
			continue
		}
		seen[row] = true
		r := &m.rows[row]
		if !r.Alive {
			continue
		}
		coefficient := m.coefficient(row, x)
		if coefficient.Sign() == 0 {
			continue
		}
		if r.Relation == Equal {
			return m.solveFor(row, x)
		}
		if r.Relation == Divides {
			modular = append(modular, row)
			continue
		}
		bound := new(big.Rat).Sub(&m.values[x], new(big.Rat).Quo(&r.Value, coefficient))
		if coefficient.Sign() > 0 {
			if upperIndex == NullVariable || bound.Cmp(upperValue) < 0 || (bound.Cmp(upperValue) == 0 && r.Relation == Less && !upperStrict) {
				upperIndex, upperValue, upperStrict = row, bound, r.Relation == Less
			}
			upper = append(upper, row)
			upperUnit = upperUnit && coefficient.Cmp(big.NewRat(1, 1)) == 0
		} else {
			if lowerIndex == NullVariable || bound.Cmp(lowerValue) > 0 || (bound.Cmp(lowerValue) == 0 && r.Relation == Less && !lowerStrict) {
				lowerIndex, lowerValue, lowerStrict = row, bound, r.Relation == Less
			}
			lower = append(lower, row)
			lowerUnit = lowerUnit && coefficient.Cmp(big.NewRat(-1, 1)) == 0
		}
	}
	if len(modular) != 0 {
		return m.solveMod(x, modular)
	}

	representative := lowerIndex
	if len(upper) <= len(lower) {
		representative = upperIndex
	}
	all := append(append([]uint32{}, lower...), upper...)
	if representative == NullVariable {
		for _, row := range all {
			m.retire(row)
		}
		return nil
	}
	if (len(upper) <= 2 || len(lower) <= 2) && len(upper) <= 3 && len(lower) <= 3 && (!m.integers[x] || upperUnit || lowerUnit) {
		for i, upperRow := range upper {
			coefficient := m.coefficient(upperRow, x)
			last := i+1 == len(upper)
			for _, lowerRow := range lower {
				destination := lowerRow
				if !last {
					var err error
					destination, err = m.copyRow(lowerRow)
					if err != nil {
						return err
					}
				}
				if err := m.resolve(upperRow, coefficient, destination, x); err != nil {
					return err
				}
			}
		}
		for _, row := range upper {
			m.retire(row)
		}
		return nil
	}
	coefficient := m.coefficient(representative, x)
	for _, row := range all {
		if row != representative {
			if err := m.resolve(representative, coefficient, row, x); err != nil {
				return err
			}
		}
	}
	m.retire(representative)
	return nil
}

func (m *ModelBasedOpt) LiveRows() []OptRow {
	var out []OptRow
	for i := range m.rows {
		if m.rows[i].Alive {
			out = append(out, m.rows[i].Clone())
		}
	}
	return out
}

func (m *ModelBasedOpt) DisplayRow(w io.Writer, row *OptRow) error {
	if row == nil {
		return fmt.Errorf("simplex: nil optimization row")
	}
	if row.Alive {
		if _, err := io.WriteString(w, "+ "); err != nil {
			return err
		}
	} else if _, err := io.WriteString(w, "- "); err != nil {
		return err
	}
	for i := range row.Terms {
		if i > 0 && row.Terms[i].Coefficient.Sign() > 0 {
			if _, err := io.WriteString(w, "+ "); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s*v%d ", row.Terms[i].Coefficient.RatString(), row.Terms[i].Variable); err != nil {
			return err
		}
	}
	if row.Constant.Sign() > 0 {
		if _, err := fmt.Fprintf(w, "+ %s ", row.Constant.RatString()); err != nil {
			return err
		}
	} else if row.Constant.Sign() < 0 {
		if _, err := fmt.Fprintf(w, "%s ", row.Constant.RatString()); err != nil {
			return err
		}
	}
	if row.Relation == Divides {
		_, err := fmt.Fprintf(w, "mod %s = 0; value: %s\n", row.Modulus.RatString(), row.Value.RatString())
		return err
	}
	_, err := fmt.Fprintf(w, "%s 0; value: %s\n", row.Relation, row.Value.RatString())
	return err
}

func (m *ModelBasedOpt) Display(w io.Writer) error {
	for i := range m.rows {
		if err := m.DisplayRow(w, &m.rows[i]); err != nil {
			return err
		}
	}
	for variable, rows := range m.incidence {
		if _, err := fmt.Fprintf(w, "%d: ", variable); err != nil {
			return err
		}
		for _, row := range rows {
			if _, err := fmt.Fprintf(w, "%d ", row); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}
