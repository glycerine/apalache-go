package ast

// GetSortNumParameters exposes sort_info::get_num_parameters for retained
// source consumers that classify parameterized declarations.
func (m *ASTManager) GetSortNumParameters(h ASTHandle) (uint32, error) {
	if err := m.lock(); err != nil {
		return 0, err
	}
	defer m.mu.Unlock()
	r, err := m.requireKind(h, ASTSort)
	if err != nil {
		return 0, err
	}
	info := m.sortAt(r.slot).info
	if info == (ASTSortInfo{}) {
		return 0, nil
	}
	v, err := m.sortInfo(info)
	if err != nil {
		return 0, err
	}
	return v.count, nil
}
