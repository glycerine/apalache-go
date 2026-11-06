package subpaving

// Source: src/util/ref_buffer.h:32-52,56-61;
// src/math/subpaving/tactic/subpaving_tactic.cpp:165-178.

import "github.com/glycerine/g3/pkg/arena"

func (w *HardwareShellCoefficients) DiscardUnreferencedInequality(a *HardwareInequality) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DiscardUnreferencedInequality(a)
}

func (w *FastShellCoefficients) DiscardUnreferencedInequality(a *FastInequality) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DiscardUnreferencedInequality(a)
}

func (w *FixedShellCoefficients) DiscardUnreferencedInequality(a *FixedInequality) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DiscardUnreferencedInequality(a)
}

func (w *ArbitraryShellCoefficients) DiscardUnreferencedInequality(a *ArbitraryInequality) error {
	if err := w.open(); err != nil {
		return err
	}
	return w.context.DiscardUnreferencedInequality(a)
}

// DiscardUnreferencedInequality completes the tactic's temporary ref_buffer
// destruction after a clause-construction failure. A published clause or unit
// owns its atom; only an atom with no surviving reference is removed.
func (c *HardwareContext) DiscardUnreferencedInequality(a *HardwareInequality) error {
	if err := c.open(); err != nil {
		return err
	}
	if a == nil || a.context != c {
		return arena.ErrInvalidSlot
	}
	if a.refs != 0 {
		return nil
	}
	for i, existing := range c.inequalities {
		if existing == a {
			copy(c.inequalities[i:], c.inequalities[i+1:])
			c.inequalities[len(c.inequalities)-1] = nil
			c.inequalities = c.inequalities[:len(c.inequalities)-1]
			a.context = nil
			return nil
		}
	}
	return arena.ErrInvalidSlot
}

func (c *FastContext) DiscardUnreferencedInequality(a *FastInequality) error {
	if err := c.open(); err != nil {
		return err
	}
	if a == nil || a.context != c || a.owner == nil {
		return arena.ErrInvalidSlot
	}
	if a.refs != 0 {
		return nil
	}
	for i, existing := range c.inequalities {
		if existing == a {
			if err := a.owner.Close(); err != nil {
				return err
			}
			copy(c.inequalities[i:], c.inequalities[i+1:])
			c.inequalities[len(c.inequalities)-1] = nil
			c.inequalities = c.inequalities[:len(c.inequalities)-1]
			a.context = nil
			a.owner = nil
			a.value = nil
			return nil
		}
	}
	return arena.ErrInvalidSlot
}

func (c *FixedContext) DiscardUnreferencedInequality(a *FixedInequality) error {
	if err := c.open(); err != nil {
		return err
	}
	if a == nil || a.context != c || a.value == nil {
		return arena.ErrInvalidSlot
	}
	if a.refs != 0 {
		return nil
	}
	for i, existing := range c.inequalities {
		if existing == a {
			if err := a.value.Close(); err != nil {
				return err
			}
			copy(c.inequalities[i:], c.inequalities[i+1:])
			c.inequalities[len(c.inequalities)-1] = nil
			c.inequalities = c.inequalities[:len(c.inequalities)-1]
			a.context = nil
			a.value = nil
			return nil
		}
	}
	return arena.ErrInvalidSlot
}

func (c *ArbitraryContext) DiscardUnreferencedInequality(a *ArbitraryInequality) error {
	if err := c.open(); err != nil {
		return err
	}
	if a == nil || a.context != c {
		return arena.ErrInvalidSlot
	}
	if a.refs != 0 {
		return nil
	}
	for i, existing := range c.inequalities {
		if existing == a {
			if err := c.numbers.Del(&a.value); err != nil {
				return err
			}
			copy(c.inequalities[i:], c.inequalities[i+1:])
			c.inequalities[len(c.inequalities)-1] = nil
			c.inequalities = c.inequalities[:len(c.inequalities)-1]
			a.context = nil
			return nil
		}
	}
	return arena.ErrInvalidSlot
}

func (c *RationalContext) DiscardUnreferencedInequality(a *RationalInequality) error {
	if err := c.open(); err != nil {
		return err
	}
	if a == nil || a.context != c || a.closed {
		return arena.ErrInvalidSlot
	}
	if a.refs != 0 {
		return nil
	}
	for i, existing := range c.inequalities {
		if existing == a {
			if err := c.numbers.Del(&a.value); err != nil {
				return err
			}
			copy(c.inequalities[i:], c.inequalities[i+1:])
			c.inequalities[len(c.inequalities)-1] = nil
			c.inequalities = c.inequalities[:len(c.inequalities)-1]
			a.context = nil
			a.closed = true
			return nil
		}
	}
	return arena.ErrInvalidSlot
}
