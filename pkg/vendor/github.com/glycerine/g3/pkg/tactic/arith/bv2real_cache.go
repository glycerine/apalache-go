package arith

// Source: src/tactic/arith/bv2real_rewriter.h:35-39,61-62;
// src/tactic/arith/bv2real_rewriter.cpp:110-129.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// The forward key and reverse value each own a separate rational pair.
// The ledger supplies deterministic cleanup without iterating either map.
type bv2RealCacheOwner struct {
	values           [4]unumeral.OwnedRational // forward d,r, then reverse d,r
	header           uint64
	reverseBytes     uint64
	reversePublished bool
	forward          bool
	retiring         bool
	retireFrom       int
}

func (u *BV2RealUtil) releaseCacheValues(entry *bv2RealCacheOwner, first int) error {
	for i := len(entry.values) - 1; i >= first; i-- {
		if entry.values[i] != (unumeral.OwnedRational{}) {
			if err := u.manager.NumeralService().Del(&entry.values[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (u *BV2RealUtil) cleanupCacheFailures() error {
	for _, entry := range u.cacheOwners {
		if entry.retiring {
			if err := u.releaseCacheValues(entry, entry.retireFrom); err != nil {
				return err
			}
			if !entry.reversePublished && entry.reverseBytes != 0 {
				u.manager.Budget().Release(entry.reverseBytes)
				entry.reverseBytes = 0
			}
			entry.retiring = false
		}
	}
	kept := u.cacheOwners[:0]
	for _, entry := range u.cacheOwners {
		if entry.forward {
			kept = append(kept, entry)
		} else {
			u.manager.Budget().Release(entry.header)
			entry.header = 0
		}
	}
	u.cacheOwners = kept
	u.cacheRetiring = false
	return nil
}
func (u *BV2RealUtil) cacheFailure(entry *bv2RealCacheOwner, err error) error {
	entry.retiring = true
	if entry.forward {
		entry.retireFrom = 2
	}
	u.cacheRetiring = true
	return errors.Join(err, u.cleanupCacheFailures())
}
func (u *BV2RealUtil) closeCacheOwners() error {
	// C++ destroys decl2sig before sig2decl, and r before d in each signature.
	for _, entry := range u.cacheOwners {
		if err := u.releaseCacheValues(entry, 2); err != nil {
			return err
		}
		u.manager.Budget().Release(entry.reverseBytes)
		entry.reverseBytes = 0
	}
	for _, entry := range u.cacheOwners {
		if err := u.releaseCacheValues(entry, 0); err != nil {
			return err
		}
		u.manager.Budget().Release(entry.header)
		entry.header = 0
	}
	u.cacheOwners = nil
	return nil
}
func (u *BV2RealUtil) publishEncoding(key bv2RealKey, decl ast.ASTHandle, widthA, widthB uint32, sourceD, sourceR *unumeral.OwnedRational) error {
	reverseBytes := uint64(reflect.TypeOf([2]unumeral.OwnedRational{}).Size())
	bytes := uint64(reflect.TypeOf(bv2RealCacheOwner{}).Size()) - reverseBytes
	if err := u.manager.Budget().Reserve(bytes); err != nil {
		return err
	}
	entry := &bv2RealCacheOwner{header: bytes}
	u.cacheOwners = append(u.cacheOwners, entry)
	service := u.manager.NumeralService()
	for i := range entry.values {
		if i == 2 {
			if err := u.manager.Budget().Reserve(reverseBytes); err != nil {
				return u.cacheFailure(entry, err)
			}
			entry.reverseBytes = reverseBytes
		}
		value, err := service.NewValue()
		if err != nil {
			return u.cacheFailure(entry, err)
		}
		entry.values[i] = value
		source := sourceD
		if i%2 == 1 {
			source = sourceR
		}
		if err = service.Copy(&entry.values[i], source); err != nil {
			return u.cacheFailure(entry, err)
		}
		if i == 1 {
			u.forward[key] = decl
			entry.forward = true
		}
	}
	signature := bv2RealSignature{first: widthA, second: widthB, divisor: &entry.values[2], root: &entry.values[3]}
	u.reverse[decl] = signature
	entry.reversePublished = true
	return nil
}

// find(f, sig) copies the reverse signature into a local rational pair.
func (c *bv2RealCall) signatureValues(signature *bv2RealSignature) (d, r big.Rat) {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	service := c.u.manager.NumeralService()
	for i, source := range []*unumeral.OwnedRational{signature.divisor, signature.root} {
		value := c.number()
		if c.err != nil {
			return
		}
		if c.err = service.Copy(value, source); c.err != nil {
			return
		}
		text, err := service.String(value)
		if err != nil {
			c.err = err
			return
		}
		target := &d
		if i == 1 {
			target = &r
		}
		if _, ok := target.SetString(text); !ok {
			c.err = ast.ErrASTDomain
			return
		}
	}
	return
}
