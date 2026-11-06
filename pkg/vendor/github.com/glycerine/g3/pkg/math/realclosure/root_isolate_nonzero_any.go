package realclosure

// Source: src/math/realclosure/realclosure.cpp:2398-2415.

import "errors"

func (o *algebraicRootOwner) isolateNonzeroAnyRoots(p []*realValue, usePrem bool) (roots []*realValue, err error) {
	if o.api != nil && !o.api.cleanDenominators {
		return o.isolateNonzeroCleanAnyRoots(p, usePrem)
	}
	normalized, denominator, err := o.values.cleanAnyPolynomialDenominators(p, usePrem)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, o.values.releaseSturmCoefficients(normalized), o.values.releaseSturmScratch(denominator))
	}()
	return o.isolateNonzeroCleanAnyRoots(normalized, usePrem)
}
