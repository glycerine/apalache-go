package rel

// SupportsCompiledFiniteTables reports whether every currently selected
// finite relation has a concrete bytecode implementation. Callers use this as
// a preflight before executing any instruction so unsupported plugins fall
// back to the interpreted evaluator without partial relation mutation.
func (m *RelationManager) SupportsCompiledFiniteTables() error {
	if err := m.open(); err != nil {
		return err
	}
	for _, table := range m.finite {
		switch table.(type) {
		case *SparseTable, *LazySparseTable, *FiniteCheckedPair:
			// Supported alongside the HashTable owners stored in m.relations.
		default:
			return ErrCompilerUnsupported
		}
	}
	return nil
}
