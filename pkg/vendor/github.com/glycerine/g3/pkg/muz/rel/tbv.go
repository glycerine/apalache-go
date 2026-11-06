package rel

// Source: src/muz/rel/tbv.h; src/muz/rel/tbv.cpp.

import (
	"errors"
	"math/big"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/bitset"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

var ErrTBV = errors.New("muz/rel: invalid ternary bit vector operation")

// TBit uses the source two-bit encoding. Bitwise or forms the union of
// possibilities and bitwise and forms their intersection.
type TBit uint8

const (
	TBitZ TBit = iota
	TBit0
	TBit1
	TBitX
)

func NegTBit(value TBit) TBit { return value ^ TBitX }

// TBV is an owning-manager-local ternary bit vector. The wrapper retains its
// physical owner because source projection and permutation may read a vector
// allocated by a differently sized manager.
type TBV struct {
	owner *TBVManager
	ref   bitset.FixedRef
	live  bool
	index int // position in the owner's live-vector ledger
}

// TBVManager owns fixed-width ternary vectors. Callers must deallocate vectors
// before Close when they need source-equivalent lifetime accounting.
type TBVManager struct {
	bits      *bitset.FixedManager
	allocated []*TBV
	closed    bool
}

func NewTBVManager(numTBits uint32, config arena.VectorConfig) (*TBVManager, error) {
	if numTBits > ^uint32(0)/2 {
		return nil, arena.ErrBounds
	}
	bits, err := bitset.NewFixedManager(2*numTBits, config)
	if err != nil {
		return nil, err
	}
	return &TBVManager{bits: bits}, nil
}

func (m *TBVManager) NumTBits() uint32 {
	if m == nil || m.bits == nil {
		return 0
	}
	return m.bits.NumBits() / 2
}

func (m *TBVManager) SizeEstimateBytes() uint32 {
	if m == nil || m.bits == nil {
		return 0
	}
	return m.bits.NumBytes()
}

func (m *TBVManager) checkOwner(vector *TBV) error {
	if m == nil || m.closed || m.bits == nil {
		return arena.ErrClosed
	}
	if vector == nil || !vector.live || vector.owner != m || vector.index < 0 || vector.index >= len(m.allocated) || m.allocated[vector.index] != vector {
		return ErrTBV
	}
	return nil
}

func checkReadable(vector *TBV) error {
	if vector == nil || !vector.live || vector.owner == nil || vector.owner.closed {
		return ErrTBV
	}
	return nil
}

func (m *TBVManager) Allocate() (*TBV, error) {
	if m == nil || m.closed || m.bits == nil {
		return nil, arena.ErrClosed
	}
	ref, err := m.bits.Allocate()
	if err != nil {
		return nil, err
	}
	result := &TBV{owner: m, ref: ref, live: true, index: len(m.allocated)}
	m.allocated = append(m.allocated, result)
	return result, nil
}

func (m *TBVManager) allocateFilled(value TBit) (*TBV, error) {
	result, err := m.Allocate()
	if err != nil {
		return nil, err
	}
	if err = m.Fill(result, value); err != nil {
		_ = m.Deallocate(result)
		return nil, err
	}
	return result, nil
}

func (m *TBVManager) Allocate0() (*TBV, error) { return m.allocateFilled(TBit0) }
func (m *TBVManager) Allocate1() (*TBV, error) { return m.allocateFilled(TBit1) }
func (m *TBVManager) AllocateX() (*TBV, error) { return m.allocateFilled(TBitX) }

func (m *TBVManager) AllocateCopy(source *TBV) (*TBV, error) {
	result, err := m.Allocate()
	if err != nil {
		return nil, err
	}
	if err = m.Copy(result, source); err != nil {
		_ = m.Deallocate(result)
		return nil, err
	}
	return result, nil
}

func (m *TBVManager) AllocateUint64(value uint64) (*TBV, error) {
	result, err := m.Allocate0()
	if err != nil {
		return nil, err
	}
	limit := m.NumTBits()
	if limit > 64 {
		limit = 64
	}
	for bit := uint32(0); bit < limit; bit++ {
		if value&(uint64(1)<<bit) != 0 {
			if err = m.Set(result, bit, TBit1); err != nil {
				_ = m.Deallocate(result)
				return nil, err
			}
		}
	}
	return result, nil
}

func (m *TBVManager) AllocateUint64Range(value uint64, high, low uint32) (*TBV, error) {
	result, err := m.AllocateX()
	if err != nil {
		return nil, err
	}
	if err = m.SetUint64(result, value, high, low); err != nil {
		_ = m.Deallocate(result)
		return nil, err
	}
	return result, nil
}

func (m *TBVManager) AllocateBigInt(value *big.Int) (*TBV, error) {
	if value == nil {
		return nil, ErrTBV
	}
	result, err := m.Allocate0()
	if err != nil {
		return nil, err
	}
	for bit := uint32(0); bit < m.NumTBits(); bit++ {
		if bigIntBit(value, bit) {
			if err = m.Set(result, bit, TBit1); err != nil {
				_ = m.Deallocate(result)
				return nil, err
			}
		}
	}
	return result, nil
}

func (m *TBVManager) AllocatePermutation(source *TBV, permutation []uint32) (*TBV, error) {
	if err := checkReadable(source); err != nil || uint32(len(permutation)) != m.NumTBits() || source.owner.NumTBits() < m.NumTBits() {
		return nil, ErrTBV
	}
	result, err := m.Allocate()
	if err != nil {
		return nil, err
	}
	for i, destination := range permutation {
		value, readErr := source.owner.Get(source, uint32(i))
		if readErr != nil {
			err = readErr
			break
		}
		if err = m.Set(result, destination, value); err != nil {
			break
		}
	}
	if err != nil {
		_ = m.Deallocate(result)
		return nil, err
	}
	return result, nil
}

func (m *TBVManager) AllocateString(text string) (*TBV, error) {
	result, err := m.AllocateX()
	if err != nil {
		return nil, err
	}
	index := uint32(0)
	for _, char := range text {
		if index >= m.NumTBits() {
			break
		}
		var value TBit
		switch char {
		case '0':
			value = TBit0
		case '1':
			value = TBit1
		case '*', 'x':
			value = TBitX
		case ' ', '\t':
			if index == 0 {
				continue
			}
			return result, nil
		default:
			return result, nil
		}
		if err = m.Set(result, index, value); err != nil {
			_ = m.Deallocate(result)
			return nil, err
		}
		index++
	}
	return result, nil
}

func (m *TBVManager) Deallocate(vector *TBV) error {
	if err := m.checkOwner(vector); err != nil {
		return err
	}
	if m.bits.NumBits() != 0 {
		if err := m.bits.Deallocate(vector.ref); err != nil {
			return err
		}
	}
	last := len(m.allocated) - 1
	moved := m.allocated[last]
	m.allocated[vector.index] = moved
	moved.index = vector.index
	m.allocated[last] = nil
	m.allocated = m.allocated[:last]
	vector.live = false
	return nil
}

func (m *TBVManager) Reset() {
	if m == nil || m.closed {
		return
	}
	m.bits.Reset()
	for _, vector := range m.allocated {
		vector.live = false
	}
	m.allocated = nil
}

func (m *TBVManager) Close() {
	if m == nil || m.closed {
		return
	}
	for _, vector := range m.allocated {
		vector.live = false
	}
	m.allocated = nil
	m.bits.Close()
	m.closed = true
}

func (m *TBVManager) Get(vector *TBV, index uint32) (TBit, error) {
	if err := m.checkOwner(vector); err != nil {
		return TBitZ, err
	}
	if index >= m.NumTBits() {
		return TBitZ, arena.ErrBounds
	}
	high, err := m.bits.Get(vector.ref, 2*index)
	if err != nil {
		return TBitZ, err
	}
	low, err := m.bits.Get(vector.ref, 2*index+1)
	if err != nil {
		return TBitZ, err
	}
	var value TBit
	if high {
		value |= 2
	}
	if low {
		value |= 1
	}
	return value, nil
}

func (m *TBVManager) Set(vector *TBV, index uint32, value TBit) error {
	if err := m.checkOwner(vector); err != nil {
		return err
	}
	if index >= m.NumTBits() || value > TBitX {
		return arena.ErrBounds
	}
	if err := m.bits.Set(vector.ref, 2*index, value&2 != 0); err != nil {
		return err
	}
	return m.bits.Set(vector.ref, 2*index+1, value&1 != 0)
}

func (m *TBVManager) Fill(vector *TBV, value TBit) error {
	if err := m.checkOwner(vector); err != nil {
		return err
	}
	if value > TBitX {
		return ErrTBV
	}
	patterns := [...]uint32{0, 0xaaaaaaaa, 0x55555555, 0xffffffff}
	for word := uint32(0); word < m.bits.NumWords(); word++ {
		if err := m.bits.WriteWord(vector.ref, word, patterns[value]); err != nil {
			return err
		}
	}
	return nil
}

func (m *TBVManager) Copy(destination, source *TBV) error {
	if err := m.checkOwner(destination); err != nil {
		return err
	}
	if err := checkReadable(source); err != nil || source.owner.NumTBits() != m.NumTBits() {
		return ErrTBV
	}
	if source.owner == m {
		return m.bits.Copy(destination.ref, source.ref)
	}
	for i := uint32(0); i < m.NumTBits(); i++ {
		value, err := source.owner.Get(source, i)
		if err != nil {
			return err
		}
		if err = m.Set(destination, i, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *TBVManager) SetOr(destination, source *TBV) error {
	if err := m.checkOwner(destination); err != nil {
		return err
	}
	if err := m.checkOwner(source); err != nil {
		return err
	}
	return m.bits.Or(destination.ref, source.ref)
}

func (m *TBVManager) SetAnd(destination, source *TBV) (bool, error) {
	if err := m.checkOwner(destination); err != nil {
		return false, err
	}
	if err := m.checkOwner(source); err != nil {
		return false, err
	}
	if err := m.bits.And(destination.ref, source.ref); err != nil {
		return false, err
	}
	return m.WellFormed(destination)
}

func (m *TBVManager) Complement(source *TBV) ([]*TBV, error) {
	if err := m.checkOwner(source); err != nil {
		return nil, err
	}
	result := make([]*TBV, 0, m.NumTBits())
	for i := uint32(0); i < m.NumTBits(); i++ {
		value, err := m.Get(source, i)
		if err != nil {
			return result, err
		}
		if value != TBit0 && value != TBit1 {
			continue
		}
		copy, err := m.AllocateCopy(source)
		if err != nil {
			return result, err
		}
		if err = m.Set(copy, i, NegTBit(value)); err != nil {
			_ = m.Deallocate(copy)
			return result, err
		}
		result = append(result, copy)
	}
	return result, nil
}

func (m *TBVManager) Equals(left, right *TBV) (bool, error) {
	if err := m.checkOwner(left); err != nil {
		return false, err
	}
	if err := m.checkOwner(right); err != nil {
		return false, err
	}
	return m.bits.Equals(left.ref, right.ref)
}

func (m *TBVManager) Hash(vector *TBV) (uint32, error) {
	if err := m.checkOwner(vector); err != nil {
		return 0, err
	}
	return m.bits.Hash(vector.ref)
}

func (m *TBVManager) Contains(left, right *TBV) (bool, error) {
	if err := m.checkOwner(left); err != nil {
		return false, err
	}
	if err := m.checkOwner(right); err != nil {
		return false, err
	}
	return m.bits.Contains(left.ref, right.ref)
}

func (m *TBVManager) ContainsColumns(left *TBV, leftColumns []uint32, right *TBV, rightColumns []uint32) (bool, error) {
	if err := m.checkOwner(left); err != nil {
		return false, err
	}
	if err := checkReadable(right); err != nil || len(leftColumns) != len(rightColumns) {
		return false, ErrTBV
	}
	for i, leftColumn := range leftColumns {
		leftValue, err := m.Get(left, leftColumn)
		if err != nil {
			return false, err
		}
		if leftValue == TBitX {
			continue
		}
		rightValue, err := right.owner.Get(right, rightColumns[i])
		if err != nil {
			return false, err
		}
		if leftValue != rightValue {
			return false, nil
		}
	}
	return true, nil
}

func (m *TBVManager) Intersect(left, right, result *TBV) (bool, error) {
	if err := m.Copy(result, left); err != nil {
		return false, err
	}
	return m.SetAnd(result, right)
}

func (m *TBVManager) WellFormed(vector *TBV) (bool, error) {
	if err := m.checkOwner(vector); err != nil {
		return false, err
	}
	for i := uint32(0); i < m.NumTBits(); i++ {
		value, err := m.Get(vector, i)
		if err != nil {
			return false, err
		}
		if value == TBitZ {
			return false, nil
		}
	}
	return true, nil
}

func (m *TBVManager) SetUint64(destination *TBV, value uint64, high, low uint32) error {
	if low > high || high >= m.NumTBits() || high-low >= 64 {
		return arena.ErrBounds
	}
	for i := uint32(0); i <= high-low; i++ {
		bit := TBit0
		if value&(uint64(1)<<i) != 0 {
			bit = TBit1
		}
		if err := m.Set(destination, low+i, bit); err != nil {
			return err
		}
	}
	return nil
}

func bigIntBit(value *big.Int, index uint32) bool {
	mask := new(big.Int).Lsh(big.NewInt(1), uint(index))
	return new(big.Int).And(new(big.Int).Set(value), mask).Sign() != 0
}

func (m *TBVManager) SetBigInt(destination *TBV, value *big.Int, high, low uint32) error {
	if value == nil || low > high || high >= m.NumTBits() {
		return arena.ErrBounds
	}
	for i := uint32(0); i <= high-low; i++ {
		bit := TBit0
		if bigIntBit(value, i) {
			bit = TBit1
		}
		if err := m.Set(destination, low+i, bit); err != nil {
			return err
		}
	}
	return nil
}

func (m *TBVManager) SetSlice(destination, source *TBV, high, low uint32) error {
	if err := m.checkOwner(destination); err != nil {
		return err
	}
	if err := checkReadable(source); err != nil || low > high || high >= m.NumTBits() || source.owner.NumTBits() < high-low+1 {
		return arena.ErrBounds
	}
	for i := uint32(0); i <= high-low; i++ {
		value, err := source.owner.Get(source, i)
		if err != nil {
			return err
		}
		if err = m.Set(destination, low+i, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *TBVManager) Project(deleted []bool, source *TBV) (*TBV, error) {
	if err := checkReadable(source); err != nil || uint32(len(deleted)) > source.owner.NumTBits() {
		return nil, ErrTBV
	}
	result, err := m.Allocate()
	if err != nil {
		return nil, err
	}
	destination := uint32(0)
	for sourceIndex, remove := range deleted {
		if remove {
			continue
		}
		if destination >= m.NumTBits() {
			err = ErrTBV
			break
		}
		value, readErr := source.owner.Get(source, uint32(sourceIndex))
		if readErr != nil {
			err = readErr
			break
		}
		if err = m.Set(result, destination, value); err != nil {
			break
		}
		destination++
	}
	if err == nil && destination != m.NumTBits() {
		err = ErrTBV
	}
	if err != nil {
		_ = m.Deallocate(result)
		return nil, err
	}
	return result, nil
}

func (m *TBVManager) DisplayRange(vector *TBV, high, low uint32) (string, error) {
	if err := m.checkOwner(vector); err != nil {
		return "", err
	}
	if low > high || high >= m.NumTBits() {
		return "", arena.ErrBounds
	}
	var output strings.Builder
	output.Grow(int(high - low + 1))
	for i := high + 1; i > low; {
		i--
		value, err := m.Get(vector, i)
		if err != nil {
			return "", err
		}
		output.WriteByte("z01x"[value])
	}
	return output.String(), nil
}

func (m *TBVManager) String(vector *TBV) (string, error) {
	if m.NumTBits() == 0 {
		if err := m.checkOwner(vector); err != nil {
			return "", err
		}
		return "[]", nil
	}
	return m.DisplayRange(vector, m.NumTBits()-1, 0)
}

func (m *TBVManager) ToFormula(manager *ast.ASTManager, vector *TBV) (ast.ASTHandle, error) {
	if manager == nil {
		return ast.ASTHandle{}, ErrTBV
	}
	if err := m.checkOwner(vector); err != nil {
		return ast.ASTHandle{}, err
	}
	booleanSort, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	conjunction := make([]ast.ASTHandle, 0, m.NumTBits())
	for i := uint32(0); i < m.NumTBits(); i++ {
		value, readErr := m.Get(vector, i)
		if readErr != nil {
			return ast.ASTHandle{}, readErr
		}
		if value != TBit0 && value != TBit1 {
			continue
		}
		variable, makeErr := manager.MkConst(usymbol.Numerical(i), booleanSort)
		if makeErr != nil {
			return ast.ASTHandle{}, makeErr
		}
		if value == TBit0 {
			variable, makeErr = manager.MkNot(variable)
			if makeErr != nil {
				return ast.ASTHandle{}, makeErr
			}
		}
		conjunction = append(conjunction, variable)
	}
	return manager.MkAndArray(conjunction)
}

func (m *TBVManager) MkVar(manager *ast.ASTManager, index uint32) (ast.ASTHandle, error) {
	if manager == nil {
		return ast.ASTHandle{}, ErrTBV
	}
	sort, err := manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkConst(usymbol.Numerical(index), sort)
}
