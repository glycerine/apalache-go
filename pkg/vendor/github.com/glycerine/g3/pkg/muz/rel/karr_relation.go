package rel

// Source: src/muz/rel/karr_relation.cpp/.h. This is the concrete Karr abstract
// relation reached by dl_mk_karr_invariants; it deliberately does not recreate
// the unused C++ relation-plugin template hierarchy.

import (
	"errors"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/math/hilbert"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var (
	ErrKarrDimension = errors.New("muz/rel: inconsistent Karr relation dimension")
	ErrKarrColumn    = errors.New("muz/rel: Karr relation column out of range")
)

// KarrMatrix represents rows A*x+b = 0 or A*x+b >= 0.
type KarrMatrix struct {
	A  [][]big.Rat
	B  []big.Rat
	Eq []bool
}

func (m *KarrMatrix) Len() int {
	if m == nil {
		return 0
	}
	return len(m.A)
}

func cloneRationals(values []big.Rat) []big.Rat {
	result := make([]big.Rat, len(values))
	for i := range values {
		result[i].Set(&values[i])
	}
	return result
}

func cloneKarrMatrix(source KarrMatrix) KarrMatrix {
	result := KarrMatrix{A: make([][]big.Rat, len(source.A)), B: make([]big.Rat, len(source.B)), Eq: append([]bool(nil), source.Eq...)}
	for i := range source.A {
		result.A[i] = cloneRationals(source.A[i])
	}
	for i := range source.B {
		result.B[i].Set(&source.B[i])
	}
	return result
}

func CloneKarrMatrix(source KarrMatrix) KarrMatrix { return cloneKarrMatrix(source) }

func (m *KarrMatrix) append(row []big.Rat, constant *big.Rat, equal bool) {
	m.A = append(m.A, cloneRationals(row))
	var value big.Rat
	if constant != nil {
		value.Set(constant)
	}
	m.B = append(m.B, value)
	m.Eq = append(m.Eq, equal)
}

type KarrRelation struct {
	dimension         uint32
	empty             bool
	inequalities      KarrMatrix
	inequalitiesValid bool
	basis             KarrMatrix
	basisValid        bool
	checkpoint        func() bool
}

func NewKarrEmpty(dimension uint32, checkpoint func() bool) *KarrRelation {
	return &KarrRelation{dimension: dimension, empty: true, inequalitiesValid: true, checkpoint: checkpoint}
}

func NewKarrFull(dimension uint32, checkpoint func() bool) *KarrRelation {
	return &KarrRelation{dimension: dimension, inequalitiesValid: true, checkpoint: checkpoint}
}

func (r *KarrRelation) Dimension() uint32 { return r.dimension }
func (r *KarrRelation) Empty() bool       { return r == nil || r.empty }

func (r *KarrRelation) Clone() *KarrRelation {
	if r == nil {
		return nil
	}
	return &KarrRelation{
		dimension:         r.dimension,
		empty:             r.empty,
		inequalities:      cloneKarrMatrix(r.inequalities),
		inequalitiesValid: r.inequalitiesValid,
		basis:             cloneKarrMatrix(r.basis),
		basisValid:        r.basisValid,
		checkpoint:        r.checkpoint,
	}
}

// AddFact changes an empty relation into the abstraction of one concrete fact.
// A nil coordinate denotes a non-integer/non-numeral source value and is skipped.
func (r *KarrRelation) AddFact(values []*big.Rat) error {
	if r == nil || uint32(len(values)) != r.dimension {
		return ErrKarrDimension
	}
	if !r.empty {
		return errors.New("muz/rel: AddFact requires an empty Karr relation")
	}
	r.empty = false
	r.inequalities = KarrMatrix{}
	r.inequalitiesValid = true
	r.basisValid = false
	for i, value := range values {
		if value == nil || !value.IsInt() {
			continue
		}
		row := make([]big.Rat, r.dimension)
		row[i].SetInt64(1)
		constant := new(big.Rat).Neg(value)
		r.inequalities.append(row, constant, true)
	}
	return nil
}

func (r *KarrRelation) ensureInequalities() error {
	if r == nil {
		return ErrKarrDimension
	}
	if r.inequalitiesValid {
		return nil
	}
	if !r.basisValid {
		return ErrKarrDimension
	}
	r.inequalities = r.dualizeHomogeneous(r.basis)
	r.inequalitiesValid = true
	return nil
}

func (r *KarrRelation) ensureBasis() error {
	if r == nil {
		return ErrKarrDimension
	}
	if r.basisValid {
		return nil
	}
	if !r.inequalitiesValid {
		return ErrKarrDimension
	}
	if r.inequalities.Len() == 0 {
		// The affine generator for true consists of one origin and signed
		// unit directions for every integer coordinate. The native path only
		// reaches Hilbert with a non-empty matrix; make the same mathematical
		// boundary explicit for the direct typed Horn interpreter.
		r.basis = KarrMatrix{}
		r.basis.append(make([]big.Rat, r.dimension), big.NewRat(1, 1), true)
		for column := uint32(0); column < r.dimension; column++ {
			positive := make([]big.Rat, r.dimension)
			positive[column].SetInt64(1)
			r.basis.append(positive, nil, true)
			negative := make([]big.Rat, r.dimension)
			negative[column].SetInt64(-1)
			r.basis.append(negative, nil, true)
		}
		r.basisValid = true
		return nil
	}
	basis, satisfiable := r.dualizeInequalities(r.inequalities)
	if !satisfiable {
		r.empty = true
		r.basis = KarrMatrix{}
		return nil
	}
	r.basis = basis
	r.basisValid = true
	return nil
}

func (r *KarrRelation) dualizeInequalities(source KarrMatrix) (KarrMatrix, bool) {
	basis := hilbert.New(r.checkpoint)
	for i := range source.A {
		bound := new(big.Rat).Neg(&source.B[i])
		var err error
		if source.Eq[i] {
			err = basis.AddEQ(source.A[i], bound)
		} else {
			err = basis.AddGE(source.A[i], bound)
		}
		if err != nil {
			return KarrMatrix{}, true
		}
	}
	for i := uint32(0); i < r.dimension; i++ {
		basis.SetIsInt(i)
	}
	result, err := basis.Saturate()
	if err != nil || result == lbool.Undef {
		return KarrMatrix{}, true
	}
	if result == lbool.False {
		return KarrMatrix{}, false
	}
	var destination KarrMatrix
	firstInitial := true
	for i := uint32(0); i < basis.BasisSize(); i++ {
		solution, initial, solutionErr := basis.BasisSolution(i)
		if solutionErr != nil {
			return KarrMatrix{}, true
		}
		if initial && firstInitial {
			destination.append(solution, big.NewRat(1, 1), true)
			firstInitial = false
		} else if !initial {
			destination.append(solution, nil, true)
		}
	}
	return destination, true
}

func (r *KarrRelation) dualizeHomogeneous(source KarrMatrix) KarrMatrix {
	if source.Len() == 0 {
		return KarrMatrix{}
	}
	basis := hilbert.New(r.checkpoint)
	for i := range source.A {
		row := cloneRationals(source.A[i])
		row = append(row, source.B[i])
		var err error
		if source.Eq[i] {
			err = basis.AddEQ(row, nil)
		} else {
			err = basis.AddGE(row, nil)
		}
		if err != nil {
			return KarrMatrix{}
		}
	}
	for i := uint32(0); i <= r.dimension; i++ {
		basis.SetIsInt(i)
	}
	result, err := basis.Saturate()
	if err != nil || result != lbool.True {
		return KarrMatrix{}
	}
	var destination KarrMatrix
	for i := uint32(0); i < basis.BasisSize(); i++ {
		solution, initial, solutionErr := basis.BasisSolution(i)
		if solutionErr != nil {
			return KarrMatrix{}
		}
		if initial || len(solution) == 0 {
			continue
		}
		constant := solution[len(solution)-1]
		destination.append(solution[:len(solution)-1], &constant, true)
	}
	return destination
}

func (r *KarrRelation) Inequalities() (KarrMatrix, error) {
	if err := r.ensureInequalities(); err != nil {
		return KarrMatrix{}, err
	}
	return cloneKarrMatrix(r.inequalities), nil
}

func (r *KarrRelation) Basis() (KarrMatrix, error) {
	if err := r.ensureBasis(); err != nil {
		return KarrMatrix{}, err
	}
	return cloneKarrMatrix(r.basis), nil
}

func validateKarrColumns(dimension uint32, columns []uint32) error {
	for _, column := range columns {
		if column >= dimension {
			return ErrKarrColumn
		}
	}
	return nil
}

func KarrJoin(left, right *KarrRelation, leftColumns, rightColumns []uint32) (*KarrRelation, error) {
	if left == nil || right == nil || len(leftColumns) != len(rightColumns) {
		return nil, ErrKarrDimension
	}
	if err := validateKarrColumns(left.dimension, leftColumns); err != nil {
		return nil, err
	}
	if err := validateKarrColumns(right.dimension, rightColumns); err != nil {
		return nil, err
	}
	result := NewKarrFull(left.dimension+right.dimension, left.checkpoint)
	if left.empty || right.empty {
		result.empty = true
		return result, nil
	}
	leftMatrix, err := left.Inequalities()
	if err != nil {
		return nil, err
	}
	rightMatrix, err := right.Inequalities()
	if err != nil {
		return nil, err
	}
	for i := range leftMatrix.A {
		row := make([]big.Rat, result.dimension)
		copy(row, leftMatrix.A[i])
		result.inequalities.append(row, &leftMatrix.B[i], leftMatrix.Eq[i])
	}
	for i := range rightMatrix.A {
		row := make([]big.Rat, result.dimension)
		copy(row[left.dimension:], rightMatrix.A[i])
		result.inequalities.append(row, &rightMatrix.B[i], rightMatrix.Eq[i])
	}
	for i := range leftColumns {
		row := make([]big.Rat, result.dimension)
		row[leftColumns[i]].SetInt64(1)
		row[left.dimension+rightColumns[i]].SetInt64(-1)
		result.inequalities.append(row, nil, true)
	}
	return result, nil
}

func (r *KarrRelation) Project(removed []uint32) (*KarrRelation, error) {
	if r == nil {
		return nil, ErrKarrDimension
	}
	if err := validateKarrColumns(r.dimension, removed); err != nil {
		return nil, err
	}
	removed = append([]uint32(nil), removed...)
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })
	for i := 1; i < len(removed); i++ {
		if removed[i] == removed[i-1] {
			return nil, ErrKarrColumn
		}
	}
	result := NewKarrFull(r.dimension-uint32(len(removed)), r.checkpoint)
	if r.empty {
		result.empty = true
		return result, nil
	}
	matrix, err := r.Basis()
	if err != nil {
		return nil, err
	}
	for i := range matrix.A {
		row := make([]big.Rat, 0, result.dimension)
		removedIndex := 0
		for column := uint32(0); column < r.dimension; column++ {
			if removedIndex < len(removed) && column == removed[removedIndex] {
				removedIndex++
				continue
			}
			row = append(row, matrix.A[i][column])
		}
		result.basis.append(row, &matrix.B[i], true)
	}
	result.basisValid = true
	result.inequalitiesValid = false
	return result, nil
}

func (r *KarrRelation) Rename(cycle []uint32) (*KarrRelation, error) {
	if r == nil || len(cycle) == 0 {
		return nil, ErrKarrDimension
	}
	if err := validateKarrColumns(r.dimension, cycle); err != nil {
		return nil, err
	}
	result := r.Clone()
	rename := func(matrix *KarrMatrix) {
		for i := range matrix.A {
			temporary := matrix.A[i][cycle[0]]
			for j := 0; j+1 < len(cycle); j++ {
				matrix.A[i][cycle[j]].Set(&matrix.A[i][cycle[j+1]])
			}
			matrix.A[i][cycle[len(cycle)-1]].Set(&temporary)
		}
	}
	if result.inequalitiesValid {
		rename(&result.inequalities)
	}
	if result.basisValid {
		rename(&result.basis)
	}
	return result, nil
}

func sameKarrRow(left []big.Rat, leftConstant *big.Rat, leftEqual bool, right []big.Rat, rightConstant *big.Rat, rightEqual bool) bool {
	if len(left) != len(right) || leftEqual != rightEqual || leftConstant.Cmp(rightConstant) != 0 {
		return false
	}
	for i := range left {
		if left[i].Cmp(&right[i]) != 0 {
			return false
		}
	}
	return true
}

// Union widens the receiver with source and reports whether its basis changed.
func (r *KarrRelation) Union(source *KarrRelation) (bool, error) {
	if r == nil || source == nil || r.dimension != source.dimension {
		return false, ErrKarrDimension
	}
	if source.empty {
		return false, nil
	}
	sourceBasis, err := source.Basis()
	if err != nil {
		return false, err
	}
	if r.empty {
		r.empty = false
		r.basis = sourceBasis
		r.basisValid = true
		r.inequalitiesValid = false
		return true, nil
	}
	if err = r.ensureBasis(); err != nil {
		return false, err
	}
	oldSize := r.basis.Len()
	for i := range sourceBasis.A {
		found := false
		for j := 0; !found && j < oldSize; j++ {
			found = sameKarrRow(sourceBasis.A[i], &sourceBasis.B[i], sourceBasis.Eq[i], r.basis.A[j], &r.basis.B[j], r.basis.Eq[j])
		}
		if !found {
			r.basis.append(sourceBasis.A[i], &sourceBasis.B[i], sourceBasis.Eq[i])
		}
	}
	changed := oldSize != r.basis.Len()
	if changed {
		r.inequalitiesValid = false
	}
	return changed, nil
}

func (r *KarrRelation) FilterIdentical(columns []uint32) error {
	if r == nil || len(columns) == 0 {
		return ErrKarrDimension
	}
	if err := validateKarrColumns(r.dimension, columns); err != nil {
		return err
	}
	if err := r.ensureInequalities(); err != nil {
		return err
	}
	for i := 1; i < len(columns); i++ {
		row := make([]big.Rat, r.dimension)
		row[columns[0]].SetInt64(1)
		row[columns[i]].SetInt64(-1)
		r.inequalities.append(row, nil, true)
	}
	r.basisValid = false
	return nil
}

// FilterEqual retains the native source behavior, including its fixed -1
// constant (the parsed value is only used as a validity guard upstream).
func (r *KarrRelation) FilterEqual(column uint32, integerValue bool) error {
	if r == nil || column >= r.dimension {
		return ErrKarrColumn
	}
	if !integerValue {
		return nil
	}
	if err := r.ensureInequalities(); err != nil {
		return err
	}
	row := make([]big.Rat, r.dimension)
	row[column].SetInt64(1)
	r.inequalities.append(row, big.NewRat(-1, 1), true)
	r.basisValid = false
	return nil
}

func (r *KarrRelation) AddConstraint(row []big.Rat, constant *big.Rat, equal bool) error {
	if r == nil || uint32(len(row)) != r.dimension {
		return ErrKarrDimension
	}
	if err := r.ensureInequalities(); err != nil {
		return err
	}
	r.empty = false
	r.inequalities.append(row, constant, equal)
	r.basisValid = false
	return nil
}
