package api

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/realclosure"
)

// Source: src/api/api_rcf.cpp:140-181.

func (c *SMTContext) rcfBinary(a, b *RCFNumber, operation uint8) (*RCFNumber, error) {
	service, err := c.checkRCF(a, b)
	if err != nil {
		return nil, err
	}
	if a.real != nil || b.real != nil {
		return c.rcfRealBinary(a, b, operation)
	}
	switch operation {
	case 0:
		value, operationErr := service.Add(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	case 1:
		value, operationErr := service.Sub(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	case 2:
		value, operationErr := service.Mul(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	case 3:
		zero, err := service.IsZero(&b.value)
		if err != nil {
			return nil, err
		}
		if zero {
			return nil, ErrRCFDivisionByZero
		}
		value, operationErr := service.Div(&a.value, &b.value)
		return c.wrapRCF(service, value, operationErr)
	default:
		return nil, ErrRCFNumberOwner
	}
}

func (c *SMTContext) rcfRealBinary(a, b *RCFNumber, operation uint8) (result *RCFNumber, err error) {
	left, temporaryLeft, err := c.realRCF(a)
	if err != nil {
		return nil, err
	}
	if temporaryLeft {
		defer func() { err = errors.Join(err, left.Close()) }()
	}
	right, temporaryRight, err := c.realRCF(b)
	if err != nil {
		return nil, err
	}
	if temporaryRight {
		defer func() { err = errors.Join(err, right.Close()) }()
	}
	values := c.realClosure
	var value *realclosure.APIValue
	switch operation {
	case 0:
		value, err = values.Add(left, right)
	case 1:
		value, err = values.Sub(left, right)
	case 2:
		value, err = values.Mul(left, right)
	case 3:
		var zero bool
		zero, err = values.IsZero(right)
		if err == nil && zero {
			return nil, ErrRCFDivisionByZero
		}
		if err == nil {
			value, err = values.Div(left, right)
		}
	default:
		return nil, ErrRCFNumberOwner
	}
	return c.wrapRealRCF(a.service, value, err)
}

func (c *SMTContext) RCFAdd(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 0) }
func (c *SMTContext) RCFSub(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 1) }
func (c *SMTContext) RCFMul(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 2) }
func (c *SMTContext) RCFDiv(a, b *RCFNumber) (*RCFNumber, error) { return c.rcfBinary(a, b, 3) }
