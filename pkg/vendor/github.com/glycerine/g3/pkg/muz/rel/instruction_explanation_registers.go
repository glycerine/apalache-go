package rel

import "errors"

// Typed explanation registers retain the source execution-context ownership
// contract without introducing a universal relation value adapter.

func (c *ExecutionContext) ExplanationRegister(index uint32) (*FiniteExplanationProduct, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if uint64(index) < uint64(len(c.registers)) && c.registers[index] != nil || hasRelationExplanationRegister(c, index) {
		return nil, ErrInstruction
	}
	if uint64(index) >= uint64(len(c.explanationRegisters)) {
		return nil, nil
	}
	value := c.explanationRegisters[index]
	if value != nil {
		if err := value.open(); err != nil {
			return nil, err
		}
	}
	return value, nil
}
func (c *ExecutionContext) SetExplanationRegister(index uint32, value *FiniteExplanationProduct) error {
	if err := c.open(); err != nil {
		return err
	}
	if index == ^uint32(0) {
		return ErrInstruction
	}
	if value != nil {
		if err := value.open(); err != nil {
			return err
		}
		if c.relations != nil && value.manager != c.relations {
			return ErrInstruction
		}
	}
	if err := c.clearRelationExplanationRegister(index); err != nil {
		return err
	}
	count := int(uint64(index) + 1)
	if len(c.registers) < count {
		c.registers = append(c.registers, make([]FiniteTable, count-len(c.registers))...)
	}
	if len(c.explanationRegisters) < count {
		c.explanationRegisters = append(c.explanationRegisters, make([]*FiniteExplanationProduct, count-len(c.explanationRegisters))...)
	}
	if previous := c.explanationRegisters[index]; previous != nil && previous != value {
		if err := previous.Close(); err != nil {
			return err
		}
	}
	if previous := c.registers[index]; previous != nil {
		if err := previous.Close(); err != nil {
			return err
		}
		c.registers[index] = nil
	}
	c.explanationRegisters[index] = value
	return nil
}
func (c *ExecutionContext) ReleaseExplanationRegister(index uint32) (*FiniteExplanationProduct, error) {
	value, err := c.ExplanationRegister(index)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, ErrInstruction
	}
	c.explanationRegisters[index] = nil
	return value, nil
}

func (i *registerInstruction) performExplanationRegister(context *ExecutionContext) (bool, error) {
	if i.kind == "dealloc" {
		return true, context.MakeEmpty(i.source)
	}
	source, err := context.ExplanationRegister(i.source)
	if err != nil {
		return false, err
	}
	switch i.kind {
	case "dealloc":
		return true, context.MakeEmpty(i.source)
	case "clone":
		copy, err := source.Clone()
		if err != nil {
			return false, err
		}
		if err = context.SetExplanationRegister(i.target, copy); err != nil {
			return false, errors.Join(err, copy.Close())
		}
		return true, nil
	case "move":
		value, err := context.ReleaseExplanationRegister(i.source)
		if err != nil {
			return false, err
		}
		if err = context.SetExplanationRegister(i.target, value); err != nil {
			return false, errors.Join(err, context.SetExplanationRegister(i.source, value))
		}
		return true, nil
	}
	return false, ErrInstruction
}
