package rel

func (t *SparseTable) Union(source, delta *SparseTable) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := source.open(); err != nil {
		return err
	}
	if delta != nil {
		if err := delta.open(); err != nil {
			return err
		}
	}
	if !sparseSameFunctionalSignature(t, source) || (delta != nil && !sparseSameFunctionalSignature(t, delta)) {
		return ErrTable
	}
	facts, e := source.Facts()
	if e != nil {
		return e
	}
	for _, f := range facts {
		added, e := t.Add(f)
		if e != nil {
			return e
		}
		if added && delta != nil {
			if _, e = delta.Add(f); e != nil {
				return e
			}
		}
	}
	return nil
}
