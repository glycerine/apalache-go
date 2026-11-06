package api

// Source: generated api_commands.cpp and src/api/api_rcf.cpp. Rational and
// algebraic values are supported. Transcendental and infinitesimal extensions
// remain unimplemented in the real-closure value engine.

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type replayRCFNumber struct {
	owner      *replayContextOwner
	value      *RCFNumber
	refs       uint32
	ownerBytes uint64
	budget     *arena.Budget
}

func (n *replayRCFNumber) close() error {
	if err := n.value.Close(); err != nil {
		return err
	}
	n.budget.Release(n.ownerBytes)
	return nil
}

func registerZ3GeneratedRCFCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{488, "Z3_rcf_del"}, {489, "Z3_rcf_mk_rational"}, {490, "Z3_rcf_mk_small_int"},
		{491, "Z3_rcf_mk_pi"}, {492, "Z3_rcf_mk_e"}, {493, "Z3_rcf_mk_infinitesimal"},
		{494, "Z3_rcf_mk_roots"},
		{495, "Z3_rcf_add"}, {496, "Z3_rcf_sub"}, {497, "Z3_rcf_mul"}, {498, "Z3_rcf_div"}, {499, "Z3_rcf_neg"}, {500, "Z3_rcf_inv"}, {501, "Z3_rcf_power"},
		{502, "Z3_rcf_lt"}, {503, "Z3_rcf_gt"}, {504, "Z3_rcf_le"}, {505, "Z3_rcf_ge"}, {506, "Z3_rcf_eq"}, {507, "Z3_rcf_neq"},
		{508, "Z3_rcf_num_to_string"}, {509, "Z3_rcf_num_to_decimal_string"}, {510, "Z3_rcf_get_numerator_denominator"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayRCFCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayRCFOne(r *Z3Replayer, owner *replayContextOwner) (uint64, error) {
	if owner.rcfOneID == 0 {
		value, err := owner.context.RCFSmallInt(1)
		if err != nil {
			return 0, err
		}
		if err = replayStoreRCF(r, owner, value); err != nil {
			return 0, err
		}
		owner.rcfOneID = r.result // One reference belongs to the source manager.
	}
	n := r.rcfNumbers[owner.rcfOneID]
	if n == nil || n.refs == ^uint32(0) {
		return 0, ErrZ3ReplayReference
	}
	n.refs++
	return owner.rcfOneID, nil
}

// A source arithmetic identity can return the same pointer with an extra
// owning reference. Preserve that identity for later delete commands.
func replayRetainRCF(r *Z3Replayer, owner *replayContextOwner, position uint32) error {
	id, err := r.Object(position)
	if err != nil {
		return err
	}
	if id == 0 {
		return r.StoreResult(0)
	}
	n := r.rcfNumbers[id]
	if n == nil || n.owner != owner || n.refs == 0 || n.refs == ^uint32(0) {
		return ErrZ3ReplayReference
	}
	n.refs++
	return r.StoreResult(id)
}

func replayStoreRCF(r *Z3Replayer, owner *replayContextOwner, value *RCFNumber) error {
	zero, err := owner.context.RCFIsZero(value)
	if err != nil {
		return errors.Join(err, value.Close())
	}
	if zero {
		return errors.Join(value.Close(), r.StoreResult(0))
	}
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayRCFNumber{}).Size())
	if err = owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	if r.rcfNumbers == nil {
		r.rcfNumbers = make(map[uint64]*replayRCFNumber)
	}
	r.nextObjectID++
	id := r.nextObjectID
	r.rcfNumbers[id] = &replayRCFNumber{owner: owner, value: value, refs: 1, ownerBytes: n, budget: owner.results.manager.Budget()}
	owner.rcfNumberIDs = append(owner.rcfNumberIDs, id)
	return r.StoreResult(id)
}

// The source uses a null numeral pointer for zero. Temporaries give the Go
// rational API a valid zero owner without assigning an identity to it.
func replayRCFAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*RCFNumber, bool, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, false, err
	}
	if id == 0 {
		value, e := owner.context.RCFSmallInt(0)
		return value, true, e
	}
	n := r.rcfNumbers[id]
	if n == nil || n.owner != owner || n.refs == 0 {
		return nil, false, ErrZ3ReplayReference
	}
	return n.value, false, nil
}

func replayRCFCommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 491 || id == 492 {
		var value *RCFNumber
		if id == 491 {
			value, err = owner.context.RCFPi()
		} else {
			value, err = owner.context.RCFE()
		}
		if err != nil {
			return err
		}
		return replayStoreRCF(r, owner, value)
	}
	if id == 493 {
		value, e := owner.context.RCFInfinitesimal()
		if e != nil {
			return e
		}
		return replayStoreRCF(r, owner, value)
	}
	if id == 494 {
		count, e := r.Uint(1)
		if e != nil {
			return e
		}
		ids, e := r.ObjectArray(2)
		if e != nil {
			return e
		}
		outputs, e := r.ObjectArray(3)
		if e != nil {
			return e
		}
		if uint32(len(ids)) < count || uint32(len(outputs)) < count {
			return ErrZ3ReplayReference
		}
		ids = ids[:count]
		coefficients := make([]*RCFNumber, len(ids))
		var zeroTemporaries []*RCFNumber
		for i, object := range ids {
			if object == 0 {
				coefficients[i], e = owner.context.RCFSmallInt(0)
				if e != nil {
					for _, z := range zeroTemporaries {
						e = errors.Join(e, z.Close())
					}
					return e
				}
				zeroTemporaries = append(zeroTemporaries, coefficients[i])
				continue
			}
			n := r.rcfNumbers[object]
			if n == nil || n.owner != owner || n.refs == 0 {
				return ErrZ3ReplayReference
			}
			coefficients[i] = n.value
		}
		roots, e := owner.context.RCFRoots(coefficients)
		for _, z := range zeroTemporaries {
			e = errors.Join(e, z.Close())
		}
		if e != nil {
			return e
		}
		result := make([]uint64, len(roots))
		for i, root := range roots {
			if e = replayStoreRCF(r, owner, root); e != nil {
				for _, remaining := range roots[i+1:] {
					e = errors.Join(e, remaining.Close())
				}
				return e
			}
			result[i] = r.result
		}
		return r.SetObjectArray(3, result)
	}
	if id == 489 || id == 490 {
		var value *RCFNumber
		if id == 489 {
			text, e := r.String(1)
			if e != nil {
				return e
			}
			value, err = owner.context.RCFRational(text)
		} else {
			n, e := r.Int(1)
			if e != nil {
				return e
			}
			value, err = owner.context.RCFSmallInt(n)
		}
		if err != nil {
			return err
		}
		return replayStoreRCF(r, owner, value)
	}
	if id == 488 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
		n := r.rcfNumbers[object]
		if n == nil || n.owner != owner || n.refs == 0 {
			return ErrZ3ReplayReference
		}
		n.refs--
		if n.refs == 0 {
			if e = n.close(); e != nil {
				n.refs++
				return e
			}
			delete(r.rcfNumbers, object)
		}
		return nil
	}
	first, temporary, err := replayRCFAt(r, owner, 1)
	if err != nil {
		return err
	}
	if temporary {
		defer func() { err = errors.Join(err, first.Close()) }()
	}
	if id == 501 {
		power, e := r.Uint(2)
		if e != nil {
			return e
		}
		if power >= 1<<31 {
			return fmt.Errorf("rcf replay: exponent exceeds terminating source range")
		}
		if power == 0 {
			one, e := replayRCFOne(r, owner)
			if e != nil {
				return e
			}
			return r.StoreResult(one)
		}
		if temporary {
			return r.StoreResult(0)
		}
		text, e := owner.context.RCFString(first)
		if e != nil {
			return e
		}
		if power == 1 || text == "1" || (text == "-1" && power%2 == 1) {
			return replayRetainRCF(r, owner, 1)
		}
		result, e := owner.context.RCFPower(first, power)
		if e != nil {
			return e
		}
		return replayStoreRCF(r, owner, result)
	}
	if id == 510 {
		if _, err = r.Object(2); err != nil {
			return err
		}
		if _, err = r.Object(3); err != nil {
			return err
		}
		text, e := owner.context.RCFString(first)
		if e != nil {
			return e
		}
		rational, ok := new(big.Rat).SetString(text)
		if !ok {
			return ErrRCFNumberOwner
		}
		var numerator, denominator uint64
		if rational.IsInt() {
			if e = replayRetainRCF(r, owner, 1); e != nil {
				return e
			}
			numerator = r.result
			denominator, e = replayRCFOne(r, owner)
			if e != nil {
				return e
			}
		} else {
			n, d, e := owner.context.RCFNumeratorDenominator(first)
			if e != nil {
				return e
			}
			if e = replayStoreRCF(r, owner, n); e != nil {
				return errors.Join(e, d.Close())
			}
			numerator = r.result
			if e = replayStoreRCF(r, owner, d); e != nil {
				return e
			}
			denominator = r.result
		}
		if e = r.SetObject(2, numerator); e != nil {
			return e
		}
		if e = r.SetObject(3, denominator); e != nil {
			return e
		}
		return r.StoreResult(0)
	}
	if id >= 495 && id <= 500 {
		if id == 499 || id == 500 {
			var result *RCFNumber
			if id == 499 {
				result, err = owner.context.RCFNeg(first)
			} else {
				result, err = owner.context.RCFInv(first)
			}
			if err != nil {
				return err
			}
			return replayStoreRCF(r, owner, result)
		}
		second, secondTemporary, e := replayRCFAt(r, owner, 2)
		if e != nil {
			return e
		}
		if secondTemporary {
			defer func() { err = errors.Join(err, second.Close()) }()
		}
		if id == 495 {
			if temporary {
				return replayRetainRCF(r, owner, 2)
			}
			if secondTemporary {
				return replayRetainRCF(r, owner, 1)
			}
		}
		if id == 496 && secondTemporary {
			return replayRetainRCF(r, owner, 1)
		}
		if id == 497 || id == 498 {
			if temporary {
				return r.StoreResult(0)
			}
			if id == 497 && secondTemporary {
				return r.StoreResult(0)
			}
			if id == 497 {
				one, e := first.service.IsOne(&first.value)
				if e != nil {
					return e
				}
				if one {
					return replayRetainRCF(r, owner, 2)
				}
			}
			if !secondTemporary {
				one, e := second.service.IsOne(&second.value)
				if e != nil {
					return e
				}
				if one {
					return replayRetainRCF(r, owner, 1)
				}
			}
		}
		var result *RCFNumber
		switch id {
		case 495:
			result, err = owner.context.RCFAdd(first, second)
		case 496:
			result, err = owner.context.RCFSub(first, second)
		case 497:
			result, err = owner.context.RCFMul(first, second)
		case 498:
			result, err = owner.context.RCFDiv(first, second)
		}
		if err != nil {
			return err
		}
		return replayStoreRCF(r, owner, result)
	}
	if id >= 502 && id <= 507 {
		second, secondTemporary, e := replayRCFAt(r, owner, 2)
		if e != nil {
			return e
		}
		if secondTemporary {
			defer func() { err = errors.Join(err, second.Close()) }()
		}
		switch id {
		case 502:
			_, err = owner.context.RCFLess(first, second)
		case 503:
			_, err = owner.context.RCFGreater(first, second)
		case 504:
			_, err = owner.context.RCFLessEqual(first, second)
		case 505:
			_, err = owner.context.RCFGreaterEqual(first, second)
		case 506:
			_, err = owner.context.RCFEqual(first, second)
		case 507:
			_, err = owner.context.RCFNotEqual(first, second)
		}
		return err
	}
	if id == 508 {
		if _, err = r.Bool(2); err != nil {
			return err
		}
		if _, err = r.Bool(3); err != nil {
			return err
		}
		_, err = owner.context.RCFString(first)
		return err
	}
	precision, err := r.Uint(2)
	if err != nil {
		return err
	}
	_, err = owner.context.RCFDecimalString(first, precision)
	return err
}
