package numeral

// Source: src/util/mpz.cpp:1385-1410; src/util/mpn.cpp:371-399.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// Concrete cold caller receipt. Tuples survive retirement; flags grant no
// authority without exact service, frame and nonwrapping source invocation.

type NumeralInsertionReceipt struct {
	service                                             *RationalService
	natural                                             *NaturalManager
	Character                                           *integerCharacterScope
	formatting                                          *naturalFormattingScope
	invocation, formattingInvocation, displayInvocation uint64
	characterOwned, characterCompleted, formattingOwned bool
	active, sourceDone, retired                         bool
	bodyCause, sourceCause, ownCause                    error
}

func (r *NumeralInsertionReceipt) Active() bool { return r != nil && r.active }
func (s *RationalService) CompleteNumeralInsertion(r *NumeralInsertionReceipt, owning bool) error {
	if e := s.lock(); e != nil {
		return e
	}
	defer s.mu.Unlock()
	return CompleteNumeralInsertion(r, s.Manager.Z.Natural, s, owning)
}
func CompleteNumeralInsertion(r *NumeralInsertionReceipt, natural *NaturalManager, service *RationalService, owning bool) error {
	if r == nil || !r.active || r.service != service || r.natural != natural || !natural.matchesIntegerCharacterScope(r.Character) || r.invocation == 0 || (r.characterOwned && r.invocation != r.Character.invocation) || r.displayInvocation != r.Character.displayInvocation {
		return arena.ErrInvalidSlot
	}
	n := r.natural
	if r.Character.callExecuting {
		return arena.ErrInvalidSlot
	}
	if r.formattingOwned && r.formatting != nil && (r.formatting != r.Character.formatting || r.formattingInvocation != r.formatting.invocation) {
		return arena.ErrInvalidSlot
	}
	stop := func(e error) error {
		if e != nil && r.ownCause == nil {
			r.ownCause = e
		}
		return e
	}
	if !owning {
		if r.sourceDone {
			return nil
		}
		if r.formattingOwned && r.formatting != nil {
			if e := n.unwindFormattingScope(r.formatting); e != nil {
				return stop(e)
			}
		}
		if r.characterOwned && r.characterCompleted {
			if e := n.destroyIntegerCharacterScope(r.Character); e != nil {
				return stop(e)
			}
			r.characterOwned = false
		}
		r.sourceDone = true
		return nil
	}
	if !r.sourceDone {
		return arena.ErrInvalidSlot
	}
	if r.formattingOwned && r.formatting != nil {
		if e := n.cleanupFormattingScope(r.formatting); e != nil {
			return stop(e)
		}
	}
	if r.characterOwned && r.Character.pending {
		r.Character.Buffer = nil
		r.Character.reset()
	}
	r.characterOwned = false
	r.formattingOwned = false
	r.active = false
	r.retired = true
	return nil
}

func matchesServiceCharacterScope(c *integerCharacterScope, n *NaturalManager) bool {
	if c == nil {
		return false
	}
	if c == &n.CharFormatting {
		return true
	}
	q := c.caller
	return q.valid() && q.integer.Natural == n && c == &q.characters
}

func completeServiceNumeralInsertion(r *NumeralInsertionReceipt, s *RationalService, owning bool) error {
	return CompleteNumeralInsertion(r, s.Manager.Z.Natural, s, owning)
}

func (r *NumeralInsertionReceipt) SourceCause() error {
	if r == nil {
		return nil
	}
	return r.sourceCause
}
func (r *NumeralInsertionReceipt) Complete(n *NaturalManager, owning bool) error {
	return CompleteNumeralInsertion(r, n, nil, owning)
}

// Owning abort of an actually issued insertion, shared by owned string callers.
func cleanupOwnedNumeralInsertion(r *NumeralInsertionReceipt, n *NaturalManager) error {
	if r.service == nil && r.natural == n && !r.characterOwned && !r.formattingOwned {
		// An ended insertion holds no child. Do not inspect a genuine later
		// character invocation merely to discard this obsolete receipt.
		r.active, r.retired = false, true
		return nil
	}
	if r.service != nil || r.natural != n || !n.matchesIntegerCharacterScope(r.Character) || r.invocation == 0 || r.displayInvocation != r.Character.displayInvocation || r.Character.callExecuting {
		return ErrNumericOwner
	}
	if r.characterOwned && r.invocation != r.Character.invocation {
		return ErrNumericOwner
	}
	if r.formattingOwned {
		if r.formatting == nil || r.formatting != r.Character.formatting || r.formattingInvocation != r.formatting.invocation {
			return ErrNumericOwner
		}
		if e := n.cleanupFormattingScope(r.formatting); e != nil {
			return e
		}
	}
	if r.characterOwned && r.Character.pending {
		r.Character.Buffer = nil
		r.Character.reset()
	}
	r.characterOwned, r.formattingOwned, r.active = false, false, false
	r.retired = true
	return nil
}
