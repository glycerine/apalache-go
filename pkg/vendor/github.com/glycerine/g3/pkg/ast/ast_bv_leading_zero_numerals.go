package ast

// Source: src/ast/rewriter/bv_rewriter.cpp:1425-1446.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"reflect"
)

// BitVectorLeadingZeroNumerals retains the original default v and each reached
// literal-two/returned-div full expression. Recursive frames retain their parent.
type BitVectorLeadingZeroNumerals struct {
	self                   *BitVectorLeadingZeroNumerals
	manager                *ASTManager
	parent                 *BitVectorLeadingZeroNumerals
	values                 [3]unumeral.OwnedRational
	placed                 [3]bool
	scalar                 unumeral.RationalServiceInvocation
	header                 uint64
	sourceCause            error
	stopped, ended, closed bool
}

func NewBitVectorLeadingZeroNumerals(m *ASTManager, parent *BitVectorLeadingZeroNumerals) (*BitVectorLeadingZeroNumerals, error) {
	if parent != nil && (parent.manager != m || parent.self != parent || parent.closed || parent.ended || parent.stopped) {
		return nil, ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(BitVectorLeadingZeroNumerals{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return nil, e
	}
	n := &BitVectorLeadingZeroNumerals{manager: m, parent: parent, header: bytes}
	n.self = n
	v, e := m.numerals.NewValue()
	if e != nil {
		return n, n.fail(e)
	}
	n.values[0] = v
	n.placed[0] = true
	return n, nil
}
func (n *BitVectorLeadingZeroNumerals) open() error {
	if n == nil || n.self != n || n.closed {
		return ErrASTHandle
	}
	if n.stopped || n.ended {
		return ErrASTPending
	}
	return nil
}
func (n *BitVectorLeadingZeroNumerals) fail(e error) error {
	if e != nil {
		n.stopped = true
	}
	return e
}
func (n *BitVectorLeadingZeroNumerals) retire() error {
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
func (n *BitVectorLeadingZeroNumerals) Read(arg ASTHandle, width *uint32) (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.BitVectorNumeralValueWithInvocation(arg, &n.values[0], width, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	return yes, n.fail(e)
}
func (n *BitVectorLeadingZeroNumerals) Parent() *BitVectorLeadingZeroNumerals { return n.parent }
func (n *BitVectorLeadingZeroNumerals) IsPositive() (bool, error) {
	if e := n.open(); e != nil {
		return false, e
	}
	yes, e := n.manager.numerals.IsPos(&n.values[0])
	return yes, n.fail(e)
}
func (n *BitVectorLeadingZeroNumerals) Step() error {
	if e := n.open(); e != nil {
		return e
	}
	if n.placed[1] || n.placed[2] {
		return ErrASTPending
	}
	q := n.manager.numerals
	v, e := q.NewValue()
	if e != nil {
		return n.fail(e)
	}
	n.values[1] = v
	n.placed[1] = true
	e = q.SetInt64WithInvocation(&n.values[1], 2, &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	v, e = q.IDivWithInvocation(&n.values[0], &n.values[1], &n.scalar)
	n.values[2] = v
	n.placed[2] = v != (unumeral.OwnedRational{})
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	e = q.CopyWithInvocation(&n.values[0], &n.values[2], &n.scalar)
	if e == nil {
		e = n.retire()
	}
	if e != nil {
		return n.fail(e)
	}
	return n.End(2, 1)
}
func (n *BitVectorLeadingZeroNumerals) End(indices ...int) error {
	if e := n.open(); e != nil {
		return e
	}
	if e := n.scalar.SourceReady(); e != nil {
		n.sourceCause = e
		return n.fail(e)
	}
	for _, i := range indices {
		if i < 0 || i >= len(n.values) {
			return ErrASTHandle
		}
		if n.placed[i] {
			if e := n.manager.numerals.Del(&n.values[i]); e != nil {
				n.sourceCause = e
				return n.fail(e)
			}
			n.placed[i] = false
		}
	}
	return nil
}

// SourceEnd ends reached division expression values before older v.
func (n *BitVectorLeadingZeroNumerals) SourceEnd() error {
	if e := n.End(2, 1, 0); e != nil {
		return e
	}
	n.ended = true
	return nil
}
func (n *BitVectorLeadingZeroNumerals) Close() error {
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
