package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:461-467; interval_def.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/cooperate"
)

func checkCooperation(owner *cooperate.Cooperation) error {
	if owner == nil {
		return arena.ErrConfig
	}
	_, err := owner.Enabled()
	return err
}

// BindCooperation borrows the source process's optional cooperation owner.
func (c *FixedContext) BindCooperation(owner *cooperate.Cooperation) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := checkCooperation(owner); err != nil {
		return err
	}
	c.cooperation = owner
	return nil
}

func (c *FastContext) BindCooperation(owner *cooperate.Cooperation) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := checkCooperation(owner); err != nil {
		return err
	}
	c.cooperation = owner
	return nil
}

func (c *ArbitraryContext) BindCooperation(owner *cooperate.Cooperation) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := checkCooperation(owner); err != nil {
		return err
	}
	c.cooperation = owner
	return nil
}

func (c *HardwareContext) BindCooperation(owner *cooperate.Cooperation) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := checkCooperation(owner); err != nil {
		return err
	}
	c.cooperation = owner
	return nil
}

func (c *RationalContext) BindCooperation(owner *cooperate.Cooperation) error {
	if err := c.open(); err != nil {
		return err
	}
	if err := checkCooperation(owner); err != nil {
		return err
	}
	c.cooperation = owner
	return nil
}
