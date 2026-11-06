package rel

func (r *CheckedRelation) Add(f Fact) (bool, error) {
	if e := r.open(); e != nil {
		return false, e
	}
	a, e := r.primary.Add(f)
	if e != nil {
		return false, e
	}
	b, e := r.checker.Add(f)
	if e != nil {
		return false, e
	}
	if a != b {
		return false, r.fail("add_fact")
	}
	return a, r.verify("add_fact")
}
func (r *CheckedRelation) Remove(f Fact) (bool, error) {
	if e := r.open(); e != nil {
		return false, e
	}
	a, e := r.primary.Remove(f)
	if e != nil {
		return false, e
	}
	b, e := r.checker.Remove(f)
	if e != nil {
		return false, e
	}
	if a != b {
		return false, r.fail("remove_fact")
	}
	return a, r.verify("remove_fact")
}
func (r *CheckedRelation) Contains(f Fact) (bool, error) {
	if e := r.open(); e != nil {
		return false, e
	}
	a, e := r.primary.Contains(f)
	if e != nil {
		return false, e
	}
	b, e := r.checker.Contains(f)
	if e != nil {
		return false, e
	}
	if a != b {
		return false, r.fail("contains fact")
	}
	return a, nil
}
func (r *CheckedRelation) Reset() error {
	if e := r.open(); e != nil {
		return e
	}
	if e := r.primary.Reset(); e != nil {
		return e
	}
	if e := r.checker.Reset(); e != nil {
		return e
	}
	return r.verify("reset")
}
func (r *CheckedRelation) Clone() (*CheckedRelation, error) {
	if e := r.verify("clone source"); e != nil {
		return nil, e
	}
	p, e := r.primary.Clone()
	if e != nil {
		return nil, e
	}
	s, e := r.checker.Clone()
	if e != nil {
		return nil, e
	}
	out := &CheckedRelation{signature: r.signature, primary: p, checker: s}
	return out, out.verify("clone")
}
