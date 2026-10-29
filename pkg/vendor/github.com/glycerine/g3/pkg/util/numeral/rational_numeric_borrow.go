package numeral

// Source: src/util/rational.cpp; src/util/scoped_numeral.h.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
)

// RationalNumericBorrow is valid only inside WithNumericBorrow's synchronous
// callback. The service lock and all lexical numeric ownership remain in util.
type RationalNumericBorrow struct {
	integerBinding, naturalBinding NumericCallerBinding
	self                           *RationalNumericBorrow
	service                        *RationalService
	active                         bool
}

func (b *RationalNumericBorrow) NumericExecuting() bool { return b.Valid() }

type rationalNumericReceiver struct{ integer *IntegerManager }

// RationalNumericLexicalReceipt identifies an accepted outer lexical call.
// Refused entry preserves the caller's prior receipt. It grants no ownership
// of a later scope on the same service.
type RationalNumericLexicalReceipt struct {
	service    *RationalService
	invocation uint64
}

// WithReceivingInteger holds the concrete receiving allocator and trace-output
// callback boundary while the service mutex is already held. Only actual host
// callback reentry is rejected; ordinary service calls continue to serialize.
func (b *RationalNumericBorrow) WithReceivingInteger(z *IntegerManager, body func() error) error {
	if !b.Valid() {
		return ErrRationalServiceOwner
	}
	if z == nil || !z.ValidOwner() || body == nil {
		return arena.ErrConfig
	}
	s := b.service
	previous := rationalNumericReceiver{}
	if v := s.numericReceiver.Load(); v != nil {
		previous = v.(rationalNumericReceiver)
	}
	s.numericReceiver.Store(rationalNumericReceiver{integer: z})
	defer s.numericReceiver.Store(previous)
	return body()
}

func (b *RationalNumericBorrow) Valid() bool {
	return b != nil && b.self == b && b.active && b.service != nil && b == &b.service.numericBorrow
}
func (s *RationalService) WithNumericBorrow(body func(*RationalNumericBorrow) error) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	if body == nil {
		return arena.ErrConfig
	}
	b := &s.numericBorrow
	if b.active {
		return ErrRationalLexicalPending
	}
	*b = RationalNumericBorrow{self: b, service: s, active: true}
	defer func() { *b = RationalNumericBorrow{} }()
	if e := s.Manager.Z.BindCaller(&b.integerBinding, b, 0); e != nil {
		return e
	}
	defer func() {
		if e := b.integerBinding.Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := s.Manager.Z.Natural.BindCaller(&b.naturalBinding, b, 0); e != nil {
		return e
	}
	defer func() {
		if e := b.naturalBinding.Unbind(); e != nil {
			panic(e)
		}
	}()
	return b.WithReceivingInteger(s.Manager.Z, func() error { return body(b) })
}
func (b *RationalNumericBorrow) Ready() error {
	if !b.Valid() {
		return ErrRationalServiceOwner
	}
	return b.service.lexicalReady()
}

// BorrowInfIntFirst is the source inf_int_rational::get_first/get_rational
// const-reference read. The returned scalar borrows its owner's integer cells
// and is valid only during this numeric-borrow callback.
func (b *RationalNumericBorrow) BorrowInfIntFirst(v *OwnedInfIntRational) (Rational, error) {
	if !b.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	if e := b.service.infIntScalarOwner(v); e != nil {
		return Rational{}, e
	}
	return v.First, nil
}

// BorrowInfFirst implements inf_rational::get_first/get_rational's borrowed
// const-reference read within this scope.
func (b *RationalNumericBorrow) BorrowInfFirst(v *OwnedInfRational) (Rational, error) {
	if !b.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	if e := b.service.infScalarOwner(v); e != nil {
		return Rational{}, e
	}
	return v.First, nil
}

// BorrowInfInfinitesimal implements inf_rational::get_infinitesimal's borrowed
// const-reference read without inspecting the unrelated FIRST component.
func (b *RationalNumericBorrow) BorrowInfInfinitesimal(v *OwnedInfRational) (Rational, error) {
	if !b.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	if e := b.service.infScalarOwner(v); e != nil {
		return Rational{}, e
	}
	return v.Second, nil
}

// InfRationalBorrow is a read-only view of the selected nested source Numeral.
// Its methods are valid only while the numeric-borrow callback is active.
type InfRationalBorrow struct {
	scope *RationalNumericBorrow
	value *OwnedInfRational
}

func (v InfRationalBorrow) First() (Rational, error) {
	if !v.scope.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	return v.value.First, nil
}

func (v InfRationalBorrow) Infinitesimal() (Rational, error) {
	if !v.scope.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	return v.value.Second, nil
}

// The inf_eps_rational accessors forward to the selected nested rational or
// infinity coefficient. Each return borrows that component for this scope.
func (b *RationalNumericBorrow) BorrowInfEpsNumeral(v *OwnedInfEpsInfRational) (InfRationalBorrow, error) {
	if !b.Valid() {
		return InfRationalBorrow{}, ErrRationalServiceOwner
	}
	if e := b.service.infEpsScalarOwner(v); e != nil {
		return InfRationalBorrow{}, e
	}
	return InfRationalBorrow{scope: b, value: &v.numeral}, nil
}

func (b *RationalNumericBorrow) BorrowInfEpsRational(v *OwnedInfEpsInfRational) (Rational, error) {
	if !b.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	if e := b.service.infEpsScalarOwner(v); e != nil {
		return Rational{}, e
	}
	return v.numeral.First, nil
}

func (b *RationalNumericBorrow) BorrowInfEpsInfinitesimal(v *OwnedInfEpsInfRational) (Rational, error) {
	if !b.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	if e := b.service.infEpsScalarOwner(v); e != nil {
		return Rational{}, e
	}
	return v.numeral.Second, nil
}

func (b *RationalNumericBorrow) BorrowInfEpsInfinity(v *OwnedInfEpsInfRational) (Rational, error) {
	if !b.Valid() {
		return Rational{}, ErrRationalServiceOwner
	}
	if e := b.service.infEpsScalarOwner(v); e != nil {
		return Rational{}, e
	}
	return v.infinity, nil
}
func (b *RationalNumericBorrow) Integers() *IntegerManager {
	if !b.Valid() {
		return nil
	}
	return b.service.Manager.Z
}
func (b *RationalNumericBorrow) WithLexical(body func() error) (cause error) {
	return b.WithLexicalReceipt(nil, body)
}
func (b *RationalNumericBorrow) WithLexicalReceipt(receipt *RationalNumericLexicalReceipt, body func() error) (cause error) {
	if !b.Valid() {
		return ErrRationalServiceOwner
	}
	if body == nil {
		return arena.ErrConfig
	}
	s := b.service
	if receipt != nil && s.Lexical.depth != 0 {
		return ErrRationalLexicalPending
	}
	if e := s.beginLexical(); e != nil {
		return e
	}
	if receipt != nil {
		*receipt = RationalNumericLexicalReceipt{service: s, invocation: s.lexicalGeneration}
	}
	defer func() {
		s.endLexicalError(&cause)
		if s.Lexical.Pending() && s.Lexical.Cause != nil && !errors.Is(cause, s.Lexical.Cause) {
			cause = errors.Join(cause, s.Lexical.Cause)
		}
	}()
	return body()
}

// CleanupAfterVectorsReceipt leaves an independently entered lexical scope
// alone, while still permitting the caller to release its own older vectors.
func (b *RationalNumericBorrow) CleanupAfterVectorsReceipt(receipt *RationalNumericLexicalReceipt, vectors func() error) error {
	if !b.Valid() {
		return ErrRationalServiceOwner
	}
	if vectors == nil {
		return arena.ErrConfig
	}
	s := b.service
	if receipt != nil && (receipt.service != s || receipt.invocation == 0 || receipt.invocation != s.lexicalGeneration) {
		return vectors()
	}
	if s.Lexical.depth != 0 {
		return ErrRationalLexicalPending
	}
	return b.CleanupAfterVectors(vectors)
}
func (b *RationalNumericBorrow) Pending() bool {
	return b.Valid() && b.service.Lexical.Cause != nil && b.service.Lexical.Pending()
}
func (b *RationalNumericBorrow) StopVectors(e error) {
	if b.Valid() && e != nil && b.service.Lexical.Pending() && b.service.Lexical.Cause == nil {
		b.service.Lexical.Cause = e
	}
}
func (b *RationalNumericBorrow) WithCopiedScalar(a *OwnedRational, body func(Rational) error) (cause error) {
	if !b.Valid() {
		return ErrRationalServiceOwner
	}
	s := b.service
	if e := s.rationalScalarOwner(a); e != nil {
		return e
	}
	if body == nil {
		return arena.ErrConfig
	}
	if e := s.beginLexical(); e != nil {
		return e
	}
	defer s.endLexicalError(&cause)
	value := NewRational()
	complete := false
	defer s.endLexicalLocal(&value, &complete)
	if e := s.Manager.Copy(&value, &a.Raw); e != nil {
		return e
	}
	complete = true
	return body(value)
}
func (b *RationalNumericBorrow) IsInt(a *Rational) (bool, error) {
	if !b.Valid() {
		return false, ErrRationalServiceOwner
	}
	return b.service.Manager.IsInt(a)
}
func (b *RationalNumericBorrow) Exponent(a *OwnedRational) (uint32, bool, error) {
	if !b.Valid() {
		return 0, false, ErrRationalServiceOwner
	}
	s := b.service
	if e := s.rationalScalarOwner(a); e != nil {
		return 0, false, e
	}
	ok, e := s.Manager.IsUnsigned(&a.Raw)
	if e != nil || !ok {
		return 0, ok, e
	}
	n, e := s.Manager.Unsigned(&a.Raw)
	return n, true, e
}

// CleanupAfterVectors finishes completed lexical scalars, then the caller's
// older vectors, and only then aborts partial scalar constructors.
func (b *RationalNumericBorrow) CleanupAfterVectors(vectors func() error) error {
	if !b.Valid() {
		return ErrRationalServiceOwner
	}
	if vectors == nil {
		return arena.ErrConfig
	}
	s := b.service
	l := &s.Lexical
	for i := uint8(0); i < l.Count; i++ {
		if l.Done&(1<<i) != 0 || l.abort[i] {
			continue
		}
		if e := lexicalDelete(s.Manager.Z, &l.Roots[i], true, &l.components[i]); e != nil {
			l.Cause = e
			return e
		}
		l.Done |= 1 << i
		for l.cursor < l.Count && l.Done&(1<<l.cursor) != 0 {
			l.cursor++
		}
	}
	if e := vectors(); e != nil {
		return e
	}
	return l.cleanup(s.Manager.Z)
}
func (b *RationalNumericBorrow) Rationals() *RationalManager {
	if !b.Valid() {
		return nil
	}
	return b.service.Manager
}
