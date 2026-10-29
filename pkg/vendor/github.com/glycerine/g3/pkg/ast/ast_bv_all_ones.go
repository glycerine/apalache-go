package ast

// Source: src/ast/bv_decl_plugin.h:387; src/ast/fpa/fpa2bv_converter.cpp:424-432.

// BitVectorNumeralAllOnes constructs the width-bit numeral 2^width-1.
func (m *ASTManager) BitVectorNumeralAllOnes(width uint32) (result ASTHandle, resultErr error) {
	if err := m.lock(); err != nil {
		return ASTHandle{}, err
	}
	defer m.mu.Unlock()
	previous := m.formatExecuting
	m.formatExecuting = true
	defer func() { m.formatExecuting = previous }()
	power, err := m.numerals.PowerOfTwo(width)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() {
		if err := m.numerals.Del(&power); resultErr == nil {
			resultErr = err
		}
	}()
	ones, err := m.numerals.SubInt(&power, 1)
	if err != nil {
		return ASTHandle{}, err
	}
	defer func() {
		if err := m.numerals.Del(&ones); resultErr == nil {
			resultErr = err
		}
	}()
	node, err := m.mkBitVectorNumeral(&ones, width)
	return m.external(node), err
}
