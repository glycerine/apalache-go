package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:2574-2589.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorRemainderEqualityNumerals retains physical divisor/rhs locals and
// separate sum/power returned values through the source condition boundary.
type BitVectorRemainderEqualityNumerals struct {
	self                   *BitVectorRemainderEqualityNumerals
	manager                *ASTManager
	values                 [4]unumeral.OwnedRational
	placed                 [4]bool
	scalar                 unumeral.RationalServiceInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorRemainderEqualityNumerals(m *ASTManager) (*BitVectorRemainderEqualityNumerals, error) {
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorRemainderEqualityNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorRemainderEqualityNumerals{manager: m, header: bytes}
	n.self = n
	for _, i := range []int{0, 1} {
		if e := n.Place(i); e != nil {
			return n, e
		}
	}
	return n, nil
}
func (n *BitVectorRemainderEqualityNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorRemainderEqualityNumerals) slot(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || !n.placed[i] {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorRemainderEqualityNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorRemainderEqualityNumerals) Place(i int) error {
	if e := n.open(); e != nil {
		return e
	}
	if i < 0 || i >= len(n.values) || n.placed[i] {
		return ErrASTPending
	}
	v, e := n.manager.numerals.NewValue()
	if e != nil {
		return n.fail(e)
	}
	n.values[i] = v
	n.placed[i] = true
	return nil
}
func (n *BitVectorRemainderEqualityNumerals) retire() error {
	if e := n.scalar.SourceReady(); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	if e := n.scalar.Cleanup(true); e != nil {
		return e
	}
	if e := n.scalar.EndContext(); e != nil {
		return e
	}
	n.scalar = unumeral.RationalServiceInvocation{}
	return nil
}
func (n *BitVectorRemainderEqualityNumerals) Read(i int, arg ASTHandle, width *uint32) (bool, error) {
	if e := n.slot(i); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[i], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}

// Outside compares the copied rhs and divisor. Valid BV numeral parameters
// are integers; the original rational comparison reaches its integer fast path.
func (n *BitVectorRemainderEqualityNumerals) Outside() (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	if e := n.slot(1); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.GreaterEqual(&n.values[1], &n.values[0])
	return yes, n.fail(e)
}
func (n *BitVectorRemainderEqualityNumerals) ReachesModulus(width uint32) (bool, error) {
	if e := n.slot(0); e != nil {
		return false, e
	}
	if e := n.slot(1); e != nil {
		return false, e
	}
	if n.placed[2] || n.placed[3] {
		return false, ErrASTPending
	}
	v, e := n.manager.numerals.AddWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return false, n.fail(e)
	}
	v, e = n.manager.numerals.PowerOfTwoWithInvocation(width, &n.scalar)
	n.values[3] = v
	n.placed[3] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return false, n.fail(e)
	}
	yes, e := n.manager.numerals.GreaterEqual(&n.values[2], &n.values[3])
	if e != nil {
		return false, n.fail(e)
	}
	// Both returned values end at the condition, before either result factory.
	return yes, n.End(3, 2)
}

// End receives explicit reverse declaration/full-expression order from the caller.
func (n *BitVectorRemainderEqualityNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	e := n.scalar.SourceReady()
	if e != nil {
		n.sourceCause = e
		return n.fail(e)
	}
	for _, i := range indices {
		if i < 0 || i >= len(n.values) {
			return ErrASTHandle
		}
		if n.placed[i] {
			if e = n.manager.numerals.Del(&n.values[i]); e != nil {
				n.sourceCause = e
				return n.fail(e)
			}
			n.placed[i] = false
		}
	}
	return nil
}
func (n *BitVectorRemainderEqualityNumerals) SourceEnd() error {
	if e := n.End(3, 2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorRemainderEqualityNumerals) Close() error {
	if n == nil || n.self != n {
		return ErrASTHandle
	}
	if n.closed {
		return nil
	}
	if e := n.scalar.Cleanup(false); e != nil {
		return e
	}
	for i := len(n.values) - 1; i >= 0; i-- {
		if n.placed[i] {
			if e := n.manager.numerals.CleanupValue(&n.values[i]); e != nil {
				return e
			}
			n.placed[i] = false
		}
	}
	if e := n.scalar.Cleanup(true); e != nil {
		return e
	}
	if e := n.scalar.EndContext(); e != nil {
		return e
	}
	n.manager.Budget().Release(n.header)
	n.header = 0
	n.closed = true
	return nil
}
