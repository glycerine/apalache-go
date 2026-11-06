package api

// Source: src/api/api_rcf.cpp:36-107. This unit owns rational and algebraic
// values, named pi/e extensions, and positive infinitesimals.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/realclosure"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (c *SMTContext) realClosureValues() (*realclosure.APIValues, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if c.realClosure == nil {
		manager, err := c.Manager()
		if err != nil {
			return nil, err
		}
		config, err := c.command.ModelEvaluatorConfig()
		if err != nil {
			return nil, err
		}
		values, err := realclosure.NewAPIValuesWithRCFParams(manager.NumeralService(), manager.ResourceLimit(), config.Cooperation, config.Context, config.Params, config.Globals)
		if err != nil {
			return nil, err
		}
		c.realClosure = values
	}
	return c.realClosure, nil
}

var ErrRCFNumberOwner = errors.New("api: invalid, foreign, or closed RCF number")
var ErrRCFDivisionByZero = errors.New("api: RCF division by zero")

// RCFNumber is the owning Go form of the source's opaque Z3_rcf_num pointer.
// Close it before closing its SMTContext.
type RCFNumber struct {
	context *smtContextState
	service *unumeral.RationalService
	value   unumeral.OwnedRational
	real    *realclosure.APIValue
	closed  bool
}

func (c *SMTContext) newRCFNumber(value unumeral.OwnedRational) (*RCFNumber, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	service := manager.NumeralService()
	if service == nil {
		return nil, ErrRCFNumberOwner
	}
	return &RCFNumber{context: c.smtContextState, service: service, value: value}, nil
}

func (c *SMTContext) RCFRational(text string) (result *RCFNumber, resultErr error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	service := manager.NumeralService()
	value, err := service.NewValue()
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, service.CleanupValue(&value))
		}
	}()
	if err = service.SetString(&value, text); err != nil {
		return nil, err
	}
	return c.newRCFNumber(value)
}

func (c *SMTContext) RCFSmallInt(value int32) (*RCFNumber, error) {
	return c.RCFRational(stringInt32(value))
}

// RCFPi returns the context's cached real-closure pi value.
func (c *SMTContext) RCFPi() (*RCFNumber, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	values, err := c.realClosureValues()
	if err != nil {
		return nil, err
	}
	value, err := values.Pi()
	return c.wrapRealRCF(manager.NumeralService(), value, err)
}

// RCFE returns the context's cached real-closure Euler constant.
func (c *SMTContext) RCFE() (*RCFNumber, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	values, err := c.realClosureValues()
	if err != nil {
		return nil, err
	}
	value, err := values.E()
	return c.wrapRealRCF(manager.NumeralService(), value, err)
}

// RCFInfinitesimal returns a fresh positive infinitesimal extension.
func (c *SMTContext) RCFInfinitesimal() (*RCFNumber, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.Manager()
	if err != nil {
		return nil, err
	}
	values, err := c.realClosureValues()
	if err != nil {
		return nil, err
	}
	value, err := values.Infinitesimal()
	return c.wrapRealRCF(manager.NumeralService(), value, err)
}

// RCFRoots returns the distinct real roots in the source manager's order.
func (c *SMTContext) RCFRoots(coefficients []*RCFNumber) ([]*RCFNumber, error) {
	if len(coefficients) == 0 {
		return nil, ErrRCFNumberOwner
	}
	service, err := c.checkRCF(coefficients...)
	if err != nil {
		return nil, err
	}
	values, err := c.realClosureValues()
	if err != nil {
		return nil, err
	}
	inputs := make([]*realclosure.APIValue, len(coefficients))
	owned := make([]*realclosure.APIValue, 0, len(coefficients))
	for i, coefficient := range coefficients {
		v, temporary, e := c.realRCF(coefficient)
		if e != nil {
			for _, x := range owned {
				_ = x.Close()
			}
			return nil, e
		}
		inputs[i] = v
		if temporary {
			owned = append(owned, v)
		}
	}
	roots, err := values.IsolateRoots(inputs)
	for _, x := range owned {
		err = errors.Join(err, x.Close())
	}
	if err != nil {
		for _, root := range roots {
			err = errors.Join(err, root.Close())
		}
		return nil, err
	}
	result := make([]*RCFNumber, len(roots))
	for i, root := range roots {
		result[i] = &RCFNumber{context: c.smtContextState, service: service, real: root}
	}
	return result, nil
}

func stringInt32(value int32) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	n := int64(value)
	if negative {
		n = -n
	}
	var buffer [11]byte
	i := len(buffer)
	for n != 0 {
		i--
		buffer[i] = byte('0' + n%10)
		n /= 10
	}
	if negative {
		i--
		buffer[i] = '-'
	}
	return string(buffer[i:])
}

func (n *RCFNumber) open() error {
	if n == nil || n.context == nil || n.service == nil || n.closed || n.context.closed {
		return ErrRCFNumberOwner
	}
	return nil
}

func (n *RCFNumber) Close() error {
	if err := n.open(); err != nil {
		return err
	}
	var err error
	if n.real != nil {
		err = n.real.Close()
	} else {
		err = n.service.CleanupValue(&n.value)
	}
	if err == nil {
		n.closed = true
		n.service = nil
		n.context = nil
	}
	return err
}
