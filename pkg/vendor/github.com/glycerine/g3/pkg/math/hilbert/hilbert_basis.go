package hilbert

// Source: src/math/hilbert/hilbert_basis.cpp/.h. The retained saturation uses
// the source saturate_orig work-list algorithm; Go slices replace heap_trie and
// C++ heap indexing without changing resolution or subsumption predicates.

import (
	"errors"
	"math"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/util/lbool"
)

var (
	ErrDimension = errors.New("math/hilbert: inconsistent inequality dimension")
	ErrOverflow  = errors.New("math/hilbert: checked int64 overflow")
)

type inequality struct {
	values []int64
	equal  bool
}

type vector struct {
	values  []int64
	weight  int64
	weights []int64
}

type Basis struct {
	inequalities []inequality
	integers     map[uint32]bool
	basis        []*vector
	current      int
	checkpoint   func() bool
	useSupport   bool
}

func New(checkpoint func() bool) *Basis {
	return &Basis{integers: make(map[uint32]bool), checkpoint: checkpoint, useSupport: true}
}

func (b *Basis) Reset() {
	b.inequalities = nil
	b.basis = nil
	b.current = 0
	b.integers = make(map[uint32]bool)
}

func checkedRat(value *big.Rat) (int64, error) {
	if value == nil || value.Denom().Cmp(big.NewInt(1)) != 0 || !value.Num().IsInt64() {
		return 0, ErrOverflow
	}
	return value.Num().Int64(), nil
}

func checkedNeg(value int64) (int64, error) {
	if value == math.MinInt64 {
		return 0, ErrOverflow
	}
	return -value, nil
}

func checkedAdd(left, right int64) (int64, error) {
	result := new(big.Int).Add(big.NewInt(left), big.NewInt(right))
	if !result.IsInt64() {
		return 0, ErrOverflow
	}
	return result.Int64(), nil
}

func checkedMul(left, right int64) (int64, error) {
	result := new(big.Int).Mul(big.NewInt(left), big.NewInt(right))
	if !result.IsInt64() {
		return 0, ErrOverflow
	}
	return result.Int64(), nil
}

func (b *Basis) add(values []big.Rat, bound *big.Rat, equal bool, negate bool) error {
	if len(b.inequalities) != 0 && len(values)+1 != len(b.inequalities[len(b.inequalities)-1].values) {
		return ErrDimension
	}
	if bound == nil {
		bound = new(big.Rat)
	}
	constant, err := checkedRat(bound)
	if err != nil {
		return err
	}
	if !negate {
		constant, err = checkedNeg(constant)
	}
	if err != nil {
		return err
	}
	row := make([]int64, len(values)+1)
	row[0] = constant
	for i := range values {
		row[i+1], err = checkedRat(&values[i])
		if err == nil && negate {
			row[i+1], err = checkedNeg(row[i+1])
		}
		if err != nil {
			return err
		}
	}
	b.inequalities = append(b.inequalities, inequality{values: row, equal: equal})
	return nil
}

func (b *Basis) AddGE(values []big.Rat, bound *big.Rat) error {
	return b.add(values, bound, false, false)
}

func (b *Basis) AddLE(values []big.Rat, bound *big.Rat) error {
	return b.add(values, bound, false, true)
}

func (b *Basis) AddEQ(values []big.Rat, bound *big.Rat) error {
	return b.add(values, bound, true, false)
}

func (b *Basis) SetIsInt(index uint32)   { b.integers[index+1] = true }
func (b *Basis) IsInt(index uint32) bool { return b.integers[index+1] }

func (b *Basis) proceed() bool { return b.checkpoint == nil || b.checkpoint() }

func (b *Basis) initBasis() {
	b.basis = nil
	if len(b.inequalities) == 0 {
		return
	}
	count := len(b.inequalities[0].values)
	for i := 0; i < count; i++ {
		values := make([]int64, count)
		values[i] = 1
		b.basis = append(b.basis, &vector{values: values})
	}
	indices := make([]int, 0, len(b.integers))
	for index := range b.integers {
		indices = append(indices, int(index))
	}
	sort.Ints(indices)
	for _, index := range indices {
		if index >= 0 && index < count {
			values := make([]int64, count)
			values[index] = -1
			b.basis = append(b.basis, &vector{values: values})
		}
	}
}

func weight(value *vector, row []int64) (int64, error) {
	var result int64
	for i := range row {
		product, err := checkedMul(value.values[i], row[i])
		if err != nil {
			return 0, err
		}
		result, err = checkedAdd(result, product)
		if err != nil {
			return 0, err
		}
	}
	return result, nil
}

func (b *Basis) selectInequality() error {
	best := b.current
	nonzero := countNonzero(b.inequalities[best].values)
	product, err := b.inequalityProduct(b.inequalities[best].values)
	if err != nil {
		return err
	}
	for i := best + 1; product != 0 && i < len(b.inequalities); i++ {
		candidateNonzero := countNonzero(b.inequalities[i].values)
		candidateProduct, productErr := b.inequalityProduct(b.inequalities[i].values)
		if productErr != nil {
			return productErr
		}
		if candidateProduct == 0 || candidateNonzero < nonzero || candidateNonzero == nonzero && candidateProduct < product {
			best, nonzero, product = i, candidateNonzero, candidateProduct
		}
	}
	if best != b.current {
		b.inequalities[best], b.inequalities[b.current] = b.inequalities[b.current], b.inequalities[best]
	}
	return nil
}

func countNonzero(values []int64) int {
	result := 0
	for _, value := range values {
		if value != 0 {
			result++
		}
	}
	return result
}

func (b *Basis) inequalityProduct(row []int64) (uint64, error) {
	var positive, negative uint64
	for _, value := range b.basis {
		w, err := weight(value, row)
		if err != nil {
			return 0, err
		}
		if w > 0 {
			positive++
		} else if w < 0 {
			negative++
		}
	}
	if positive != 0 && negative > math.MaxUint64/positive {
		return math.MaxUint64, nil
	}
	return positive * negative, nil
}

func abs64(value int64) (uint64, error) {
	if value == math.MinInt64 {
		return 0, ErrOverflow
	}
	if value < 0 {
		value = -value
	}
	return uint64(value), nil
}

func vectorMagnitude(value *vector) (uint64, error) {
	var result uint64
	for _, coordinate := range value.values {
		magnitude, err := abs64(coordinate)
		if err != nil || math.MaxUint64-result < magnitude {
			if err == nil {
				err = ErrOverflow
			}
			return 0, err
		}
		result += magnitude
	}
	return result, nil
}

func sign(value int64) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

func (b *Basis) canResolve(left, right *vector) bool {
	if sign(left.weight) == sign(right.weight) {
		return false
	}
	if left.values[0] == 1 && right.values[0] == 1 {
		return false
	}
	for index := range b.integers {
		if sign(left.values[index]) != 0 && sign(left.values[index]) == -sign(right.values[index]) {
			return false
		}
	}
	return true
}

func resolve(left, right *vector) (*vector, error) {
	result := &vector{values: make([]int64, len(left.values)), weights: make([]int64, len(left.weights))}
	var err error
	for i := range result.values {
		result.values[i], err = checkedAdd(left.values[i], right.values[i])
		if err != nil {
			return nil, err
		}
	}
	result.weight, err = checkedAdd(left.weight, right.weight)
	if err != nil {
		return nil, err
	}
	for i := range result.weights {
		result.weights[i], err = checkedAdd(left.weights[i], right.weights[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func absGE(left, right int64) bool {
	if right < 0 {
		return left <= right
	}
	return left >= right
}

func (b *Basis) subsumes(candidate, existing *vector) bool {
	if candidate == existing || candidate.weight < existing.weight || existing.weight < 0 && candidate.weight != existing.weight {
		return false
	}
	for i := range candidate.values {
		if !absGE(candidate.values[i], existing.values[i]) {
			return false
		}
	}
	for i := range candidate.weights {
		if candidate.weights[i] < existing.weights[i] {
			return false
		}
	}
	return true
}

func (b *Basis) isSubsumed(candidate *vector, accepted []*vector) bool {
	for _, existing := range accepted {
		if b.subsumes(candidate, existing) {
			return true
		}
	}
	return false
}

func (b *Basis) saturateOne(row []int64, equal bool) (lbool.LBool, error) {
	active := make([]*vector, 0)
	passive := make([]*vector, 0)
	zero := make([]*vector, 0)
	accepted := make([]*vector, 0)
	support := make(map[*vector]bool)
	addGoal := func(value *vector) bool {
		if b.isSubsumed(value, accepted) {
			return false
		}
		accepted = append(accepted, value)
		if value.weight == 0 {
			zero = append(zero, value)
		} else {
			passive = append(passive, value)
		}
		return true
	}
	for _, value := range b.basis {
		var err error
		value.weight, err = weight(value, row)
		if err != nil {
			return lbool.Undef, err
		}
		value.weights = make([]int64, b.current)
		for i := 0; i < b.current; i++ {
			value.weights[i], err = weight(value, b.inequalities[i].values)
			if err != nil {
				return lbool.Undef, err
			}
		}
		addGoal(value)
		if b.useSupport {
			support[value] = true
		}
	}
	for len(passive) != 0 {
		if !b.proceed() {
			return lbool.Undef, nil
		}
		best := 0
		bestMagnitude, err := vectorMagnitude(passive[0])
		if err != nil {
			return lbool.Undef, err
		}
		for i := 1; i < len(passive); i++ {
			magnitude, magnitudeErr := vectorMagnitude(passive[i])
			if magnitudeErr != nil {
				return lbool.Undef, magnitudeErr
			}
			if magnitude < bestMagnitude {
				best, bestMagnitude = i, magnitude
			}
		}
		candidate := passive[best]
		passive[best] = passive[len(passive)-1]
		passive = passive[:len(passive)-1]
		others := acceptedWithout(accepted, candidate)
		if b.isSubsumed(candidate, others) {
			// The source recycles the candidate here, which also removes it
			// from the subsumption index. Keep the slice index equivalent.
			accepted = others
			continue
		}
		for _, prior := range active {
			if (!b.useSupport || support[prior]) && b.canResolve(candidate, prior) {
				resolved, resolveErr := resolve(candidate, prior)
				if resolveErr != nil {
					return lbool.Undef, resolveErr
				}
				addGoal(resolved)
			}
		}
		active = append(active, candidate)
	}
	b.basis = append([]*vector(nil), zero...)
	if !equal {
		for _, value := range active {
			if value.weight > 0 {
				b.basis = append(b.basis, value)
			}
		}
	}
	if len(b.basis) == 0 {
		return lbool.False, nil
	}
	return lbool.True, nil
}

func acceptedWithout(values []*vector, excluded *vector) []*vector {
	result := make([]*vector, 0, len(values)-1)
	for _, value := range values {
		if value != excluded {
			result = append(result, value)
		}
	}
	return result
}

func (b *Basis) Saturate() (lbool.LBool, error) {
	b.initBasis()
	b.current = 0
	for b.current < len(b.inequalities) {
		if !b.proceed() {
			return lbool.Undef, nil
		}
		if err := b.selectInequality(); err != nil {
			return lbool.Undef, err
		}
		result, err := b.saturateOne(b.inequalities[b.current].values, b.inequalities[b.current].equal)
		if err != nil || result != lbool.True {
			return result, err
		}
		b.current++
	}
	if !b.proceed() {
		return lbool.Undef, nil
	}
	return lbool.True, nil
}

func (b *Basis) BasisSize() uint32 { return uint32(len(b.basis)) }

func (b *Basis) BasisSolution(index uint32) ([]big.Rat, bool, error) {
	if uint64(index) >= uint64(len(b.basis)) {
		return nil, false, ErrDimension
	}
	value := b.basis[index]
	result := make([]big.Rat, len(value.values)-1)
	for i := 1; i < len(value.values); i++ {
		result[i-1].SetInt64(value.values[i])
	}
	return result, value.values[0] != 0, nil
}

func (b *Basis) Inequality(index uint32) ([]big.Rat, big.Rat, bool, error) {
	if uint64(index) >= uint64(len(b.inequalities)) {
		return nil, big.Rat{}, false, ErrDimension
	}
	row := b.inequalities[index]
	values := make([]big.Rat, len(row.values)-1)
	for i := 1; i < len(row.values); i++ {
		values[i-1].SetInt64(row.values[i])
	}
	var bound big.Rat
	bound.SetInt64(-row.values[0])
	return values, bound, row.equal, nil
}
