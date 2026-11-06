package rel

// Source: dl_instruction.cpp execution-context ownership, specialized to the
// registered whole-relation explanation product.

func hasRelationExplanationRegister(context *ExecutionContext, index uint32) bool {
	return context != nil && index != VoidRegister && uint64(index) < uint64(len(context.relationExplanationRegisters)) && context.relationExplanationRegisters[index] != nil
}

func (c *ExecutionContext) RelationExplanationRegister(index uint32) (*RelationExplanationProduct, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	if uint64(index) < uint64(len(c.registers)) && c.registers[index] != nil || hasExplanationInstructionRegister(c, index) {
		return nil, ErrInstruction
	}
	if !hasRelationExplanationRegister(c, index) {
		return nil, nil
	}
	product := c.relationExplanationRegisters[index]
	return product, product.open()
}

func (c *ExecutionContext) SetRelationExplanationRegister(index uint32, product *RelationExplanationProduct) error {
	if err := c.open(); err != nil {
		return err
	}
	if index == VoidRegister {
		return ErrInstruction
	}
	if product != nil {
		if err := product.open(); err != nil {
			return err
		}
		if c.relations != nil && product.manager != c.relations {
			return ErrInstruction
		}
	}
	count := int(uint64(index) + 1)
	if len(c.registers) < count {
		c.registers = append(c.registers, make([]FiniteTable, count-len(c.registers))...)
	}
	if len(c.relationExplanationRegisters) < count {
		c.relationExplanationRegisters = append(c.relationExplanationRegisters, make([]*RelationExplanationProduct, count-len(c.relationExplanationRegisters))...)
	}
	if prior := c.relationExplanationRegisters[index]; prior != nil && prior != product {
		if err := prior.Close(); err != nil {
			return err
		}
	}
	if hasExplanationInstructionRegister(c, index) {
		if err := c.explanationRegisters[index].Close(); err != nil {
			return err
		}
		c.explanationRegisters[index] = nil
	}
	if prior := c.registers[index]; prior != nil {
		if err := prior.Close(); err != nil {
			return err
		}
		c.registers[index] = nil
	}
	c.relationExplanationRegisters[index] = product
	return nil
}

func (c *ExecutionContext) ReleaseRelationExplanationRegister(index uint32) (*RelationExplanationProduct, error) {
	product, err := c.RelationExplanationRegister(index)
	if err != nil {
		return nil, err
	}
	if product == nil {
		return nil, ErrInstruction
	}
	c.relationExplanationRegisters[index] = nil
	return product, nil
}

func (c *ExecutionContext) clearRelationExplanationRegister(index uint32) error {
	if hasRelationExplanationRegister(c, index) {
		if err := c.relationExplanationRegisters[index].Close(); err != nil {
			return err
		}
		c.relationExplanationRegisters[index] = nil
	}
	return nil
}
