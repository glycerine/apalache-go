package polynomial

// Source: src/math/polynomial/upolynomial_factorization.cpp:1058-1094.

import (
	primegen "github.com/glycerine/g3/pkg/util/prime"
)

// Metadata begins before p's real constructor. No NUM is created by this adapter.

func (s *UPolynomialFactorizationService) preparePrimeSearch(fs *UPolynomialFactors) (failure error) {
	q, e := s.beginPrimePreparation(fs)
	if e != nil {
		return e
	}
	r := q.receipt
	q.enterCall()
	defer q.leaveCall()
	defer func() { q.executing = false; failure = q.finish(&r, failure) }()
	c, m := q.owner, q.owner.manager
	nm := m.modular
	c.constructed = 2
	context := numericIntegerModularManager_primePreparationAllocationContext(nm, q)
	if e = nm.WithAllocationContext((context).utility(nm.Integers())).SetInt32(&c.p, 2); e != nil {
		return e
	}
	zp, e := NewRelatedUPolynomialManager(m)
	if e != nil {
		return e
	}
	c.zp = zp
	c.constructed = 3
	if e = q.bindRelated(zp); e != nil {
		return e
	}
	if e = zp.modular.WithAllocationContext((numericIntegerModularManager_primePreparationAllocationContext(zp.modular, q)).utility(zp.modular.Integers())).SetZP(&c.p); e != nil {
		return e
	}
	best, e := zp.NewFactors()
	if e != nil {
		return e
	}
	c.best = best
	c.constructed = 4
	c.constructed = 5
	if e = nm.WithAllocationContext((context).utility(nm.Integers())).SetInt32(&c.bestP, 2); e != nil {
		return e
	}
	c.constructed = 6
	// Preserve the existing concrete adaptation of the ill-formed source
	// degree_set(zp_upm) through the actual already-created EMPTY zp_fs.
	if e = c.ds.init(best); e != nil {
		return e
	}
	iterator, e := primegen.NewIterator(s.config.Primes)
	if e != nil {
		return e
	}
	c.iterator = iterator
	c.constructed = 7
	return nil
}

// A false route is the literal max-prime early publication/return. A true
// route is the first gcd==1 candidate; rejected primes repeat the checkpoint.
func (s *UPolynomialFactorizationService) nextPreparedPrime(fs *UPolynomialFactors, k uint32, params UPolynomialFactorParams) (eligible bool, failure error) {
	q, e := s.beginPrimePreparation(fs)
	if e != nil {
		return false, e
	}
	r := q.receipt
	q.enterCall()
	defer q.leaveCall()
	defer func() { q.executing = false; failure = q.finish(&r, failure) }()
	c, m := q.owner, q.owner.manager
	nm := m.modular
	context := numericIntegerModularManager_primePreparationAllocationContext(nm, q)
	for {
		if e = m.checkpoint(); e != nil {
			return false, e
		}
		next, e := c.iterator.Next()
		if e != nil {
			return false, e
		}
		if next > uint64(params.MaxPrime) {
			return false, fs.PushCopy(m.stateView(&c.fpp), k)
		}
		if e = nm.WithAllocationContext((context).utility(nm.Integers())).SetUint64(&c.p, next); e != nil {
			return false, e
		}
		if e = c.zp.modular.WithAllocationContext((numericIntegerModularManager_primePreparationAllocationContext(c.zp.modular, q)).utility(c.zp.modular.Integers())).SetZP(&c.p); e != nil {
			return false, e
		}
		top, e := m.read(m.stateView(&c.fpp), c.fpp.size-1)
		if e != nil {
			return false, e
		}
		if e = nm.WithAllocationContext((context).utility(nm.Integers())).Gcd(&c.p, top, &c.gcdTmp); e != nil {
			return false, e
		}
		if e = s.traceFactorPrimeGCD(top); e != nil {
			return false, e
		}
		one, e := nm.IsOne(&c.gcdTmp)
		if e != nil {
			return false, e
		}
		if !one {
			continue
		}
		return true, nil
	}
}
