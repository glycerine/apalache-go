package api

// Source: src/api/api_rcf.cpp:36-44,140-267.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/realclosure"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// realRCF borrows an algebraic value or makes a temporary value for a
// rational operand. The caller closes temporary values after the operation.
func (c *SMTContext) realRCF(value *RCFNumber) (*realclosure.APIValue, bool, error) {
	if _, err := c.checkRCF(value); err != nil {
		return nil, false, err
	}
	if value.real != nil {
		return value.real, false, nil
	}
	values, err := c.realClosureValues()
	if err != nil {
		return nil, false, err
	}
	text, err := value.service.String(&value.value)
	if err != nil {
		return nil, false, err
	}
	converted, err := values.Rational(text)
	return converted, err == nil, err
}

func (c *SMTContext) wrapRealRCF(service *unumeral.RationalService, value *realclosure.APIValue, err error) (*RCFNumber, error) {
	if err != nil {
		return nil, err
	}
	if err = c.open(); err != nil {
		return nil, errors.Join(err, value.Close())
	}
	return &RCFNumber{context: c.smtContextState, service: service, real: value}, nil
}

func (c *SMTContext) checkRCF(values ...*RCFNumber) (*unumeral.RationalService, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	var service *unumeral.RationalService
	for _, value := range values {
		if err := value.open(); err != nil || value.context != c.smtContextState {
			return nil, ErrRCFNumberOwner
		}
		if service == nil {
			service = value.service
		} else if service != value.service {
			return nil, ErrRCFNumberOwner
		}
	}
	if service == nil {
		return nil, ErrRCFNumberOwner
	}
	return service, nil
}

// RCFIsZero follows the source numeral-zero query for rational and extension
// values alike.
func (c *SMTContext) RCFIsZero(value *RCFNumber) (bool, error) {
	if _, err := c.checkRCF(value); err != nil {
		return false, err
	}
	if value.real != nil {
		return c.realClosure.IsZero(value.real)
	}
	return value.service.IsZero(&value.value)
}

func (c *SMTContext) wrapRCF(service *unumeral.RationalService, value unumeral.OwnedRational, operationErr error) (*RCFNumber, error) {
	if operationErr != nil {
		return nil, operationErr
	}
	if err := c.open(); err != nil {
		_ = service.CleanupValue(&value)
		return nil, err
	}
	return &RCFNumber{context: c.smtContextState, service: service, value: value}, nil
}
