package api

// Source: src/api/api_rcf.cpp:293-306.

import (
	"errors"
	"math/big"
)

func (c *SMTContext) RCFNumeratorDenominator(value *RCFNumber) (numerator, denominator *RCFNumber, resultErr error) {
	if _, err := c.checkRCF(value); err != nil {
		return nil, nil, err
	}
	if value.real != nil {
		cleanNumerator, cleanDenominator, err := c.realClosure.CleanDenominators(value.real)
		if err != nil {
			return nil, nil, err
		}
		numerator, err = c.wrapRealRCF(value.service, cleanNumerator, nil)
		if err != nil {
			return nil, nil, errors.Join(err, cleanDenominator.Close())
		}
		denominator, err = c.wrapRealRCF(value.service, cleanDenominator, nil)
		if err != nil {
			return nil, nil, errors.Join(err, numerator.Close())
		}
		return numerator, denominator, nil
	}
	text, err := c.RCFString(value)
	if err != nil {
		return nil, nil, err
	}
	rational, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, nil, ErrRCFNumberOwner
	}
	numerator, err = c.RCFRational(rational.Num().String())
	if err != nil {
		return nil, nil, err
	}
	denominator, err = c.RCFRational(rational.Denom().String())
	if err != nil {
		_ = numerator.Close()
		return nil, nil, err
	}
	return numerator, denominator, nil
}
