package rel

// Source: src/muz/rel/doc.h union_bvec<doc_manager, doc> retained concrete
// instantiation; src/muz/rel/doc.cpp complement and subtract.

import (
	"errors"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/arena"
)

// DocUnion is the concrete source udoc: an owning union of DOC values.
type DocUnion struct {
	owner  *DocManager
	values []*Doc
}

func NewDocUnion(manager *DocManager) (*DocUnion, error) {
	if manager == nil || manager.closed {
		return nil, arena.ErrClosed
	}
	return &DocUnion{owner: manager}, nil
}

func (u *DocUnion) check() error {
	if u == nil || u.owner == nil || u.owner.closed {
		return arena.ErrClosed
	}
	return nil
}

func (u *DocUnion) Len() uint32 {
	if u == nil {
		return 0
	}
	return uint32(len(u.values))
}

func (u *DocUnion) Empty() bool { return u == nil || len(u.values) == 0 }

func (u *DocUnion) At(index uint32) (*Doc, error) {
	if err := u.check(); err != nil {
		return nil, err
	}
	if index >= uint32(len(u.values)) {
		return nil, arena.ErrBounds
	}
	return u.values[index], nil
}

func (u *DocUnion) PushOwned(document *Doc) error {
	if err := u.check(); err != nil {
		return err
	}
	if err := u.owner.check(document); err != nil {
		return err
	}
	u.values = append(u.values, document)
	return nil
}

func (u *DocUnion) erase(index int) error {
	if index < 0 || index >= len(u.values) {
		return arena.ErrBounds
	}
	if err := u.owner.Deallocate(u.values[index]); err != nil {
		return err
	}
	copy(u.values[index:], u.values[index+1:])
	u.values = u.values[:len(u.values)-1]
	return nil
}

func (u *DocUnion) Reset() (result error) {
	if err := u.check(); err != nil {
		return err
	}
	for _, document := range u.values {
		result = errors.Join(result, u.owner.Deallocate(document))
	}
	u.values = nil
	return result
}

// InsertOwned consumes document whether it is inserted or subsumed.
func (u *DocUnion) InsertOwned(document *Doc) (bool, error) {
	if err := u.check(); err != nil {
		return false, err
	}
	if err := u.owner.check(document); err != nil {
		return false, err
	}
	found := false
	write := 0
	for _, existing := range u.values {
		contains, err := u.owner.Contains(existing, document)
		if err != nil {
			return false, err
		}
		if contains {
			found = true
		} else {
			contains, err = u.owner.Contains(document, existing)
			if err != nil {
				return false, err
			}
			if contains {
				if err = u.owner.Deallocate(existing); err != nil {
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
		if err := u.owner.Deallocate(document); err != nil {
			return false, err
		}
		return false, nil
	}
	u.values = append(u.values, document)
	return true, nil
}

func (u *DocUnion) Contains(document *Doc) (bool, error) {
	if err := u.check(); err != nil {
		return false, err
	}
	if err := u.owner.check(document); err != nil {
		return false, err
	}
	for _, existing := range u.values {
		contains, err := u.owner.Contains(existing, document)
		if err != nil || contains {
			return contains, err
		}
	}
	return false, nil
}

func (u *DocUnion) IsFull() (bool, error) {
	if err := u.check(); err != nil {
		return false, err
	}
	if len(u.values) != 1 {
		return false, nil
	}
	return u.owner.IsFull(u.values[0])
}

func (u *DocUnion) Copy(source *DocUnion) error {
	if err := u.check(); err != nil {
		return err
	}
	if source == nil || source.owner != u.owner {
		return ErrDOC
	}
	if err := u.Reset(); err != nil {
		return err
	}
	for _, document := range source.values {
		copy, err := u.owner.AllocateCopy(document)
		if err != nil {
			return err
		}
		u.values = append(u.values, copy)
	}
	return nil
}

func (u *DocUnion) IntersectDoc(document *Doc) error {
	if err := u.check(); err != nil {
		return err
	}
	if err := u.owner.check(document); err != nil {
		return err
	}
	write := 0
	for _, existing := range u.values {
		valid, err := u.owner.SetAnd(existing, document)
		if err != nil {
			return err
		}
		if !valid {
			if err = u.owner.Deallocate(existing); err != nil {
				return err
			}
			continue
		}
		u.values[write] = existing
		write++
	}
	u.values = u.values[:write]
	return nil
}

func (u *DocUnion) Intersect(source *DocUnion) error {
	if err := u.check(); err != nil {
		return err
	}
	if source == nil || source.owner != u.owner {
		return ErrDOC
	}
	temporary, _ := NewDocUnion(u.owner)
	result, _ := NewDocUnion(u.owner)
	for _, document := range source.values {
		if err := temporary.Copy(u); err != nil {
			_ = temporary.Reset()
			_ = result.Reset()
			return err
		}
		if err := temporary.IntersectDoc(document); err != nil {
			_ = temporary.Reset()
			_ = result.Reset()
			return err
		}
		result.values = append(result.values, temporary.values...)
		temporary.values = nil
	}
	if err := u.Reset(); err != nil {
		_ = result.Reset()
		return err
	}
	u.values, result.values = result.values, nil
	return nil
}

func (u *DocUnion) SubtractDoc(document *Doc) error {
	if err := u.check(); err != nil {
		return err
	}
	result, _ := NewDocUnion(u.owner)
	for _, existing := range u.values {
		if err := u.owner.Subtract(existing, document, result); err != nil {
			_ = result.Reset()
			return err
		}
	}
	if err := u.Reset(); err != nil {
		_ = result.Reset()
		return err
	}
	u.values, result.values = result.values, nil
	return nil
}

func (u *DocUnion) Subtract(source *DocUnion) error {
	if err := u.check(); err != nil {
		return err
	}
	if source == nil || source.owner != u.owner {
		return ErrDOC
	}
	for _, document := range source.values {
		if len(u.values) == 0 {
			break
		}
		if err := u.SubtractDoc(document); err != nil {
			return err
		}
	}
	return nil
}

func (u *DocUnion) Complement(result *DocUnion) error {
	if err := u.check(); err != nil {
		return err
	}
	if result == nil || result.owner != u.owner {
		return ErrDOC
	}
	if err := result.Reset(); err != nil {
		return err
	}
	full, err := u.owner.AllocateX()
	if err != nil {
		return err
	}
	result.values = append(result.values, full)
	for _, document := range u.values {
		negated, _ := NewDocUnion(u.owner)
		if err = u.owner.Complement(document, negated); err != nil {
			_ = negated.Reset()
			return err
		}
		if err = result.Intersect(negated); err != nil {
			_ = negated.Reset()
			return err
		}
		if err = negated.Reset(); err != nil {
			return err
		}
	}
	return nil
}

func (u *DocUnion) Simplify() error {
	if err := u.check(); err != nil {
		return err
	}
	result, _ := NewDocUnion(u.owner)
	for _, document := range u.values {
		valid, err := u.owner.FoldNeg(document)
		if err != nil {
			return err
		}
		if valid {
			if _, err = result.InsertOwned(document); err != nil {
				return err
			}
		} else if err = u.owner.Deallocate(document); err != nil {
			return err
		}
	}
	u.values = result.values
	result.values = nil
	return nil
}

func (u *DocUnion) WellFormed() (bool, error) {
	if err := u.check(); err != nil {
		return false, err
	}
	for _, document := range u.values {
		valid, err := u.owner.WellFormed(document)
		if err != nil || !valid {
			return valid, err
		}
	}
	return true, nil
}

func (u *DocUnion) SizeEstimateBytes() (uint32, error) {
	if err := u.check(); err != nil {
		return 0, err
	}
	size := uint64(len(u.values)) * uint64(reflect.TypeOf((*Doc)(nil)).Size())
	for _, document := range u.values {
		bytes, err := u.owner.SizeEstimateBytes(document)
		if err != nil {
			return 0, err
		}
		size += uint64(bytes)
	}
	if size > uint64(^uint32(0)) {
		return 0, arena.ErrBounds
	}
	return uint32(size), nil
}

func (u *DocUnion) String() (string, error) {
	if err := u.check(); err != nil {
		return "", err
	}
	if u.owner.NumTBits() == 0 {
		return "[]", nil
	}
	var output strings.Builder
	output.WriteByte('{')
	if len(u.values)+int(u.owner.NumTBits()) > 10 {
		output.WriteString("\n   ")
	}
	for i, document := range u.values {
		text, err := u.owner.String(document)
		if err != nil {
			return "", err
		}
		output.WriteString(text)
		if i+1 < len(u.values) {
			output.WriteString(", ")
			if u.owner.NumTBits() > 10 {
				output.WriteString("\n   ")
			}
		}
	}
	output.WriteByte('}')
	return output.String(), nil
}

func (m *DocManager) Complement(source *Doc, result *DocUnion) error {
	if err := m.check(source); err != nil {
		return err
	}
	if result == nil || result.owner != m {
		return ErrDOC
	}
	if err := result.Reset(); err != nil {
		return err
	}
	full, err := m.IsFull(source)
	if err != nil || full {
		return err
	}
	first, err := m.AllocateX()
	if err != nil {
		return err
	}
	negative, err := m.vectors.AllocateCopy(source.positive)
	if err != nil {
		_ = m.Deallocate(first)
		return err
	}
	first.negative.values = append(first.negative.values, negative)
	result.values = append(result.values, first)
	for _, excluded := range source.negative.values {
		document, copyErr := m.AllocateTBVCopy(excluded)
		if copyErr != nil {
			return copyErr
		}
		result.values = append(result.values, document)
	}
	return nil
}

func (m *DocManager) Subtract(left, right *Doc, result *DocUnion) error {
	if err := m.check(left); err != nil {
		return err
	}
	if err := m.check(right); err != nil {
		return err
	}
	if result == nil || result.owner != m {
		return ErrDOC
	}
	first, err := m.AllocateCopy(left)
	if err != nil {
		return err
	}
	intersection, err := m.vectors.AllocateCopy(right.positive)
	if err != nil {
		_ = m.Deallocate(first)
		return err
	}
	valid, err := m.vectors.SetAnd(intersection, left.positive)
	if err != nil {
		_ = m.vectors.Deallocate(intersection)
		_ = m.Deallocate(first)
		return err
	}
	if valid {
		if _, err = first.negative.insert(m.vectors, intersection); err != nil {
			_ = m.Deallocate(first)
			return err
		}
	} else if err = m.vectors.Deallocate(intersection); err != nil {
		_ = m.Deallocate(first)
		return err
	}
	valid, err = m.FoldNeg(first)
	if err != nil {
		_ = m.Deallocate(first)
		return err
	}
	if valid {
		result.values = append(result.values, first)
	} else {
		_ = m.Deallocate(first)
	}
	for _, excluded := range right.negative.values {
		document, copyErr := m.AllocateCopy(left)
		if copyErr != nil {
			return copyErr
		}
		valid, copyErr = m.SetAndTBV(document, excluded)
		if copyErr != nil {
			_ = m.Deallocate(document)
			return copyErr
		}
		if valid {
			result.values = append(result.values, document)
		} else {
			_ = m.Deallocate(document)
		}
	}
	return nil
}
