package sls

// Source: src/tactic/sls/sls_powers.h.
// Christoph Winter, 2012-02-29.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/numeral"
)

var errPowersOwner = errors.New("sls: invalid or closed powers owner")

// powers lazily owns one integer for every requested power of two. Returned
// pointers are borrowed const views: callers must not mutate or delete them,
// and they remain valid only until closeChecked.
type powers struct {
	manager *numeral.IntegerManager
	byShift map[uint32]*numeral.Int
	owned   []*numeral.Int
	closing int
	closed  bool
}

func newPowers(manager *numeral.IntegerManager) (*powers, error) {
	if manager == nil || manager.Closed() {
		return nil, errPowersOwner
	}
	return &powers{manager: manager, byShift: make(map[uint32]*numeral.Int)}, nil
}

func (p *powers) borrow(shift uint32) (*numeral.Int, error) {
	if p == nil || p.closed || p.manager == nil || p.manager.Closed() {
		return nil, errPowersOwner
	}
	if value := p.byShift[shift]; value != nil {
		return value, nil
	}

	value := new(numeral.Int)
	one := numeral.NewInt(1)
	if err := p.manager.Mul2k(&one, shift, value); err != nil {
		if cleanup := p.manager.Del(value); cleanup != nil {
			return nil, errors.Join(err, cleanup)
		}
		return nil, err
	}
	p.byShift[shift] = value
	p.owned = append(p.owned, value)
	return value, nil
}

// closeChecked translates the source destructor. It is retryable because a
// checked numerical owner may stop while releasing an individual cached mpz.
func (p *powers) closeChecked() error {
	if p == nil || p.closed {
		return nil
	}
	if p.manager == nil || p.manager.Closed() {
		return errPowersOwner
	}
	for p.closing < len(p.owned) {
		if err := p.manager.Del(p.owned[p.closing]); err != nil {
			return err
		}
		p.closing++
	}
	p.byShift = nil
	p.owned = nil
	p.manager = nil
	p.closed = true
	return nil
}
