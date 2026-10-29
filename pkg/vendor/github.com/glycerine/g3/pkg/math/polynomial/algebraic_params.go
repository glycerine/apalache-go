package polynomial

// Source: src/math/polynomial/algebraic_params.hpp; algebraic_numbers.cpp:150-159.

import uparams "github.com/glycerine/g3/pkg/util/params"

func CollectAlgebraicParamDescriptions(d *uparams.ParamDescrs) error {
	return catalogCollectAlgebraicParams(d)
}

func (m *AlgebraicManager) readConstructorParams() (cause error) {
	o := &m.params
	o.Running = true
	defer func() { o.Running = false }()
	f, e := o.BeginHelper(m.config.Params, "algebraic")
	if f != nil {
		defer func() { cause = o.FinishHelper(f, cause) }()
	}
	if e != nil {
		return e
	}
	v, e := o.FrameUInt(f, "min_mag", 16)
	if e != nil {
		return e
	}
	m.AlgebraicRootCore.minMagnitude = -int32(v)
	factor, e := o.FrameBool(f, "factor", true)
	if e != nil {
		return e
	}
	m.AlgebraicRootCore.factor = factor
	v, e = o.FrameUInt(f, "factor_max_prime", 31)
	if e != nil {
		return e
	}
	m.AlgebraicRootCore.factorParams.MaxPrime = v
	v, e = o.FrameUInt(f, "factor_num_primes", 1)
	if e != nil {
		return e
	}
	m.AlgebraicRootCore.factorParams.PrimeTrials = v
	v, e = o.FrameUInt(f, "factor_search_size", 5000)
	if e != nil {
		return e
	}
	m.AlgebraicRootCore.factorParams.MaxSearchSize = v
	v, e = o.FrameUInt(f, "zero_accuracy", 0)
	if e != nil {
		return e
	}
	m.AlgebraicRootCore.zeroAccuracy = -int32(v)
	return nil
}
