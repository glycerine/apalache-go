package rel

// Source: src/muz/rel/dl_relation_manager.cpp default_table_filter_equal_fn
// and default_table_filter_identical_fn, used when the sparse table plugin
// does not provide a specialized filter.

func (t *SparseTable) FilterEqual(value Element, column int) error {
	if err := t.open(); err != nil {
		return err
	}
	if column < 0 || column >= t.signature.Len() {
		return ErrTable
	}
	facts, err := t.Facts()
	if err != nil {
		return err
	}
	removed := make([]Fact, 0)
	for _, fact := range facts {
		if fact[column] != value {
			removed = append(removed, fact)
		}
	}
	_, err = t.RemoveFacts(removed)
	return err
}

func (t *SparseTable) FilterIdentical(columns []int) error {
	if err := t.open(); err != nil {
		return err
	}
	for _, column := range columns {
		if column < 0 || column >= t.signature.Len() {
			return ErrTable
		}
	}
	if len(columns) < 2 {
		return nil
	}
	facts, err := t.Facts()
	if err != nil {
		return err
	}
	removed := make([]Fact, 0)
	for _, fact := range facts {
		value := fact[columns[0]]
		for _, column := range columns[1:] {
			if fact[column] != value {
				removed = append(removed, fact)
				break
			}
		}
	}
	_, err = t.RemoveFacts(removed)
	return err
}
