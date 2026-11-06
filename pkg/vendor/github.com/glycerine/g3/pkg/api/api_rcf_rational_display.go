package api

import "strings"

// Source: src/api/api_rcf.cpp:269-291.

func (c *SMTContext) RCFString(value *RCFNumber) (string, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return "", err
	}
	if value.real != nil {
		var out strings.Builder
		if err = c.realClosure.Display(&out, value.real, true, false); err != nil {
			return "", err
		}
		return out.String(), nil
	}
	return service.String(&value.value)
}

func (c *SMTContext) RCFDecimalString(value *RCFNumber, precision uint32) (string, error) {
	service, err := c.checkRCF(value)
	if err != nil {
		return "", err
	}
	if value.real != nil {
		var out strings.Builder
		if err = c.realClosure.DisplayDecimal(&out, value.real, precision); err != nil {
			return "", err
		}
		return out.String(), nil
	}
	return service.DecimalString(&value.value, precision, false)
}
