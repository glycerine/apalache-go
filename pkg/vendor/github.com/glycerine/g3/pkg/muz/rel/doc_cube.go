package rel

// Source: src/muz/rel/doc.h concrete doc, utbv and udoc instantiations;
// src/muz/rel/doc.cpp.

import (
	"errors"
	"math/big"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
)

var ErrDOC = errors.New("muz/rel: invalid difference-of-cubes operation")

type Doc struct {
	owner      *DocManager
	positive   *TBV
	negative   tbvUnion
	headerSize uint64
	live       bool
}

func (d *Doc) Positive() *TBV {
	if d == nil || !d.live {
		return nil
	}
	return d.positive
}

func (d *Doc) NumNegative() uint32 {
	if d == nil || !d.live {
		return 0
	}
	return uint32(len(d.negative.values))
}

func (d *Doc) Negative(index uint32) (*TBV, error) {
	if d == nil || !d.live || index >= uint32(len(d.negative.values)) {
		return nil, arena.ErrBounds
	}
	return d.negative.values[index], nil
}

type tbvUnion struct{ values []*TBV }

func (u *tbvUnion) reset(manager *TBVManager) (result error) {
	for _, value := range u.values {
		result = errors.Join(result, manager.Deallocate(value))
	}
	u.values = nil
	return result
}

func (u *tbvUnion) erase(manager *TBVManager, index int) error {
	if index < 0 || index >= len(u.values) {
		return arena.ErrBounds
	}
	if err := manager.Deallocate(u.values[index]); err != nil {
		return err
	}
	copy(u.values[index:], u.values[index+1:])
	u.values = u.values[:len(u.values)-1]
	return nil
}

// insert consumes value on both the inserted and subsumed paths.
func (u *tbvUnion) insert(manager *TBVManager, value *TBV) (bool, error) {
	if _, err := manager.String(value); err != nil {
		return false, err
	}
	found := false
	write := 0
	for _, existing := range u.values {
		contains, err := manager.Contains(existing, value)
		if err != nil {
			return false, err
		}
		if contains {
			found = true
		} else {
			contains, err = manager.Contains(value, existing)
			if err != nil {
				return false, err
			}
			if contains {
				if err = manager.Deallocate(existing); err != nil {
					return false, err
				}
				continue
			}
		}
		u.values[write] = existing
		write++
	}
	u.values = u.values[:write]
	if found {
		if err := manager.Deallocate(value); err != nil {
			return false, err
		}
		return false, nil
	}
	u.values = append(u.values, value)
	return true, nil
}

func (u *tbvUnion) intersect(manager *TBVManager, cube *TBV) error {
	write := 0
	for _, value := range u.values {
		wellFormed, err := manager.SetAnd(value, cube)
		if err != nil {
			return err
		}
		if !wellFormed {
			if err = manager.Deallocate(value); err != nil {
				return err
			}
			continue
		}
		u.values[write] = value
		write++
	}
	u.values = u.values[:write]
	return nil
}

type DocManager struct {
	vectors   *TBVManager
	full      *TBV
	budget    *arena.Budget
	documents []*Doc
	closed    bool
}

func NewDocManager(width uint32, config arena.VectorConfig) (*DocManager, error) {
	vectors, err := NewTBVManager(width, config)
	if err != nil {
		return nil, err
	}
	full, err := vectors.AllocateX()
	if err != nil {
		vectors.Close()
		return nil, err
	}
	return &DocManager{vectors: vectors, full: full, budget: config.Descriptors.Budget}, nil
}

func (m *DocManager) TBVManager() *TBVManager {
	if m == nil || m.closed {
		return nil
	}
	return m.vectors
}

func (m *DocManager) NumTBits() uint32 {
	if m == nil || m.closed {
		return 0
	}
	return m.vectors.NumTBits()
}

func (m *DocManager) check(document *Doc) error {
	if m == nil || m.closed || m.vectors == nil {
		return arena.ErrClosed
	}
	if document == nil || !document.live || document.owner != m {
		return ErrDOC
	}
	return nil
}

func (m *DocManager) adopt(positive *TBV) (*Doc, error) {
	if m == nil || m.closed {
		return nil, arena.ErrClosed
	}
	if _, err := m.vectors.String(positive); err != nil {
		return nil, err
	}
	header := uint64(reflect.TypeOf(Doc{}).Size())
	if err := m.budget.Reserve(header); err != nil {
		_ = m.vectors.Deallocate(positive)
		return nil, err
	}
	document := &Doc{owner: m, positive: positive, headerSize: header, live: true}
	m.documents = append(m.documents, document)
	return document, nil
}

func (m *DocManager) Allocate() (*Doc, error) {
	value, err := m.vectors.Allocate()
	if err != nil {
		return nil, err
	}
	return m.adopt(value)
}

func (m *DocManager) Allocate0() (*Doc, error) {
	value, err := m.vectors.Allocate0()
	if err != nil {
		return nil, err
	}
	return m.adopt(value)
}

func (m *DocManager) Allocate1() (*Doc, error) {
	value, err := m.vectors.Allocate1()
	if err != nil {
		return nil, err
	}
	return m.adopt(value)
}

func (m *DocManager) AllocateX() (*Doc, error) {
	value, err := m.vectors.AllocateX()
	if err != nil {
		return nil, err
	}
	return m.adopt(value)
}

func (m *DocManager) AllocateUint64(value uint64) (*Doc, error) {
	vector, err := m.vectors.AllocateUint64(value)
	if err != nil {
		return nil, err
	}
	return m.adopt(vector)
}

func (m *DocManager) AllocateBigInt(value *big.Int) (*Doc, error) {
	vector, err := m.vectors.AllocateBigInt(value)
	if err != nil {
		return nil, err
	}
	return m.adopt(vector)
}

func (m *DocManager) AllocateUint64Range(value uint64, high, low uint32) (*Doc, error) {
	vector, err := m.vectors.AllocateUint64Range(value, high, low)
	if err != nil {
		return nil, err
	}
	return m.adopt(vector)
}

// AdoptTBV consumes an already allocated vector on success and header failure.
func (m *DocManager) AdoptTBV(vector *TBV) (*Doc, error) { return m.adopt(vector) }

func (m *DocManager) AllocateTBVCopy(vector *TBV) (*Doc, error) {
	copy, err := m.vectors.AllocateCopy(vector)
	if err != nil {
		return nil, err
	}
	return m.adopt(copy)
}

func (m *DocManager) AllocateCopy(source *Doc) (*Doc, error) {
	if err := m.check(source); err != nil {
		return nil, err
	}
	positive, err := m.vectors.AllocateCopy(source.positive)
	if err != nil {
		return nil, err
	}
	result, err := m.adopt(positive)
	if err != nil {
		return nil, err
	}
	for _, negative := range source.negative.values {
		copy, copyErr := m.vectors.AllocateCopy(negative)
		if copyErr != nil {
			_ = m.Deallocate(result)
			return nil, copyErr
		}
		result.negative.values = append(result.negative.values, copy)
	}
	return result, nil
}

func (m *DocManager) AllocatePermutation(source *Doc, permutation []uint32) (*Doc, error) {
	if err := m.check(source); err != nil {
		return nil, err
	}
	positive, err := m.vectors.AllocatePermutation(source.positive, permutation)
	if err != nil {
		return nil, err
	}
	result, err := m.adopt(positive)
	if err != nil {
		return nil, err
	}
	for _, negative := range source.negative.values {
		copy, copyErr := m.vectors.AllocatePermutation(negative, permutation)
		if copyErr != nil {
			_ = m.Deallocate(result)
			return nil, copyErr
		}
		result.negative.values = append(result.negative.values, copy)
	}
	return result, nil
}

func (m *DocManager) Deallocate(document *Doc) (result error) {
	if document == nil {
		return nil
	}
	if err := m.check(document); err != nil {
		return err
	}
	result = errors.Join(result, m.vectors.Deallocate(document.positive))
	result = errors.Join(result, document.negative.reset(m.vectors))
	document.live = false
	document.positive = nil
	m.budget.Release(document.headerSize)
	document.headerSize = 0
	return result
}

func (m *DocManager) Close() {
	if m == nil || m.closed {
		return
	}
	for i := len(m.documents); i > 0; i-- {
		if m.documents[i-1].live {
			_ = m.Deallocate(m.documents[i-1])
		}
	}
	_ = m.vectors.Deallocate(m.full)
	m.full = nil
	m.vectors.Close()
	m.documents = nil
	m.closed = true
}

func (m *DocManager) AddNegativeOwned(document *Doc, negative *TBV) error {
	if err := m.check(document); err != nil {
		return err
	}
	if _, err := m.vectors.String(negative); err != nil {
		return err
	}
	document.negative.values = append(document.negative.values, negative)
	return nil
}

func (m *DocManager) InsertNegativeOwned(document *Doc, negative *TBV) (bool, error) {
	if err := m.check(document); err != nil {
		return false, err
	}
	return document.negative.insert(m.vectors, negative)
}

func (m *DocManager) Copy(destination, source *Doc) error {
	if err := m.check(destination); err != nil {
		return err
	}
	if err := m.check(source); err != nil {
		return err
	}
	if err := m.vectors.Copy(destination.positive, source.positive); err != nil {
		return err
	}
	if err := destination.negative.reset(m.vectors); err != nil {
		return err
	}
	for _, negative := range source.negative.values {
		copy, err := m.vectors.AllocateCopy(negative)
		if err != nil {
			return err
		}
		destination.negative.values = append(destination.negative.values, copy)
	}
	return nil
}

func (m *DocManager) Fill(document *Doc, value TBit) error {
	if err := m.check(document); err != nil {
		return err
	}
	if err := document.negative.reset(m.vectors); err != nil {
		return err
	}
	return m.vectors.Fill(document.positive, value)
}

func (m *DocManager) Fill0(document *Doc) error { return m.Fill(document, TBit0) }
func (m *DocManager) Fill1(document *Doc) error { return m.Fill(document, TBit1) }
func (m *DocManager) FillX(document *Doc) error { return m.Fill(document, TBitX) }

func (m *DocManager) SizeEstimateBytes(document *Doc) (uint32, error) {
	if err := m.check(document); err != nil {
		return 0, err
	}
	size := uint64(document.headerSize) + uint64(m.vectors.SizeEstimateBytes())*uint64(1+len(document.negative.values)) + uint64(reflect.TypeOf((*TBV)(nil)).Size())*uint64(len(document.negative.values))
	if size > uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(size), nil
}

func (m *DocManager) WellFormed(document *Doc) (bool, error) {
	if err := m.check(document); err != nil {
		return false, err
	}
	valid, err := m.vectors.WellFormed(document.positive)
	if err != nil || !valid {
		return valid, err
	}
	for _, negative := range document.negative.values {
		valid, err = m.vectors.WellFormed(negative)
		if err != nil || !valid {
			return valid, err
		}
		contains, containsErr := m.vectors.Contains(document.positive, negative)
		if containsErr != nil || !contains {
			return false, containsErr
		}
	}
	return true, nil
}

func (m *DocManager) diffBy012(positive, negative *TBV) (uint32, uint32, error) {
	var index, count uint32
	for i := uint32(0); i < m.NumTBits(); i++ {
		left, err := m.vectors.Get(positive, i)
		if err != nil {
			return 0, 0, err
		}
		right, err := m.vectors.Get(negative, i)
		if err != nil {
			return 0, 0, err
		}
		if left == TBitZ || right == TBitZ {
			return 0, 0, ErrDOC
		}
		if left == right {
			continue
		}
		if count == 1 {
			return 2, index, nil
		}
		if left == TBitX {
			index, count = i, 1
		} else if right != TBitX {
			return 3, index, nil
		}
	}
	return count, index, nil
}

func (m *DocManager) FoldNeg(document *Doc) (bool, error) {
	if err := m.check(document); err != nil {
		return false, err
	}
	for {
		restart := false
		for i := 0; i < len(document.negative.values); i++ {
			negative := document.negative.values[i]
			contains, err := m.vectors.Contains(negative, document.positive)
			if err != nil {
				return false, err
			}
			if contains {
				return false, nil
			}
			count, index, err := m.diffBy012(document.positive, negative)
			if err != nil {
				return false, err
			}
			switch count {
			case 0:
				return false, nil
			case 1:
				value, err := m.vectors.Get(negative, index)
				if err != nil {
					return false, err
				}
				if err = m.vectors.Set(document.positive, index, NegTBit(value)); err != nil {
					return false, err
				}
				if err = document.negative.intersect(m.vectors, document.positive); err != nil {
					return false, err
				}
				restart = true
			case 3:
				if err = document.negative.erase(m.vectors, i); err != nil {
					return false, err
				}
				i--
			}
			if restart {
				break
			}
		}
		if !restart {
			return m.WellFormed(document)
		}
	}
}

func (m *DocManager) Set(document *Doc, index uint32, value TBit) error {
	if err := m.check(document); err != nil {
		return err
	}
	if err := m.vectors.Set(document.positive, index, value); err != nil {
		return err
	}
	for i := 0; i < len(document.negative.values); i++ {
		negative := document.negative.values[i]
		bit, err := m.vectors.Get(negative, index)
		if err != nil {
			return err
		}
		if bit != TBitX && value != TBitX && value != bit {
			if err = document.negative.erase(m.vectors, i); err != nil {
				return err
			}
			i--
		} else if err = m.vectors.Set(negative, index, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *DocManager) SetAndTBV(destination *Doc, source *TBV) (bool, error) {
	if err := m.check(destination); err != nil {
		return false, err
	}
	wellFormed, err := m.vectors.SetAnd(destination.positive, source)
	if err != nil || !wellFormed {
		return wellFormed, err
	}
	if err = destination.negative.intersect(m.vectors, source); err != nil {
		return false, err
	}
	return m.FoldNeg(destination)
}

func (m *DocManager) SetAnd(destination, source *Doc) (bool, error) {
	if err := m.check(destination); err != nil {
		return false, err
	}
	if err := m.check(source); err != nil {
		return false, err
	}
	wellFormed, err := m.vectors.SetAnd(destination.positive, source.positive)
	if err != nil || !wellFormed {
		return wellFormed, err
	}
	if err = destination.negative.intersect(m.vectors, destination.positive); err != nil {
		return false, err
	}
	for _, negative := range source.negative.values {
		copy, copyErr := m.vectors.AllocateCopy(negative)
		if copyErr != nil {
			return false, copyErr
		}
		valid, andErr := m.vectors.SetAnd(copy, destination.positive)
		if andErr != nil {
			_ = m.vectors.Deallocate(copy)
			return false, andErr
		}
		if valid {
			if _, andErr = destination.negative.insert(m.vectors, copy); andErr != nil {
				return false, andErr
			}
		} else if andErr = m.vectors.Deallocate(copy); andErr != nil {
			return false, andErr
		}
	}
	return m.FoldNeg(destination)
}

func (m *DocManager) Intersect(left, right, result *Doc) (bool, error) {
	if err := m.Copy(result, left); err != nil {
		return false, err
	}
	return m.SetAnd(result, right)
}

func (m *DocManager) Equal(left, right *Doc) (bool, error) {
	if err := m.check(left); err != nil {
		return false, err
	}
	if err := m.check(right); err != nil {
		return false, err
	}
	equal, err := m.vectors.Equals(left.positive, right.positive)
	if err != nil || !equal || len(left.negative.values) != len(right.negative.values) {
		return equal && len(left.negative.values) == len(right.negative.values), err
	}
	for i := range left.negative.values {
		equal, err = m.vectors.Equals(left.negative.values[i], right.negative.values[i])
		if err != nil || !equal {
			return equal, err
		}
	}
	return true, nil
}

func (m *DocManager) IsFull(document *Doc) (bool, error) {
	if err := m.check(document); err != nil {
		return false, err
	}
	if len(document.negative.values) != 0 {
		return false, nil
	}
	return m.vectors.Equals(document.positive, m.full)
}

func (m *DocManager) Hash(document *Doc) (uint32, error) {
	if err := m.check(document); err != nil {
		return 0, err
	}
	var result uint32
	for _, negative := range document.negative.values {
		hash, err := m.vectors.Hash(negative)
		if err != nil {
			return 0, err
		}
		result = 2*result + hash
	}
	positive, err := m.vectors.Hash(document.positive)
	return result + positive, err
}

func (m *DocManager) Contains(left, right *Doc) (bool, error) {
	if err := m.check(left); err != nil {
		return false, err
	}
	if err := m.check(right); err != nil {
		return false, err
	}
	contains, err := m.vectors.Contains(left.positive, right.positive)
	if err != nil || !contains {
		return contains, err
	}
	for _, leftNegative := range left.negative.values {
		found := false
		for _, rightNegative := range right.negative.values {
			found, err = m.vectors.Contains(rightNegative, leftNegative)
			if err != nil || found {
				break
			}
		}
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}

func (m *DocManager) ContainsColumns(left *Doc, leftColumns []uint32, right *Doc, rightColumns []uint32) (bool, error) {
	if err := m.check(left); err != nil {
		return false, err
	}
	if err := m.check(right); err != nil {
		return false, err
	}
	contains, err := m.vectors.ContainsColumns(left.positive, leftColumns, right.positive, rightColumns)
	if err != nil || !contains {
		return contains, err
	}
	for _, leftNegative := range left.negative.values {
		found := false
		for _, rightNegative := range right.negative.values {
			found, err = m.vectors.ContainsColumns(rightNegative, rightColumns, leftNegative, leftColumns)
			if err != nil || found {
				break
			}
		}
		if err != nil || !found {
			return false, err
		}
	}
	return true, nil
}

func (m *DocManager) DisplayRange(document *Doc, high, low uint32) (string, error) {
	if err := m.check(document); err != nil {
		return "", err
	}
	positive, err := m.vectors.DisplayRange(document.positive, high, low)
	if err != nil || len(document.negative.values) == 0 {
		return positive, err
	}
	var output strings.Builder
	output.WriteString(positive)
	output.WriteString(" \\ {")
	for i, negative := range document.negative.values {
		if i != 0 {
			output.WriteString(", ")
		}
		text, displayErr := m.vectors.DisplayRange(negative, high, low)
		if displayErr != nil {
			return "", displayErr
		}
		output.WriteString(text)
	}
	output.WriteByte('}')
	return output.String(), nil
}

func (m *DocManager) String(document *Doc) (string, error) {
	if m.NumTBits() == 0 {
		if err := m.check(document); err != nil {
			return "", err
		}
		return "[]", nil
	}
	return m.DisplayRange(document, m.NumTBits()-1, 0)
}
