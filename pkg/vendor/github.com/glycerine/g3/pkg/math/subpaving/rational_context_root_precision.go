package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:499-503,1641,
// concrete config_mpq inverse-root precision and interval dispatch.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/interval"
)

func (c *RationalContext) UpdateRootPrecision(denominator uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	if denominator == 0 {
		denominator = 1
	}
	if err := c.numbers.SetInt32(&c.rootPrecision, int32(denominator)); err != nil {
		return err
	}
	return c.numbers.InvInPlace(&c.rootPrecision)
}

func (c *RationalContext) XnEqY(out, src *RationalInterval, degree uint32) error {
	if err := c.open(); err != nil {
		return err
	}
	err := out.XnEqY(src, degree, &c.rootPrecision, interval.RationalRoot{Numbers: c.numbers, Limit: c.limit, Cooperation: c.cooperation})
	if errors.Is(err, interval.ErrRationalRootCanceled) {
		return ErrRationalContextCanceled
	}
	return err
}
