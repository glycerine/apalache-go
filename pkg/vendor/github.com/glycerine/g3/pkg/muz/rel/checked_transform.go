package rel

func (r *CheckedRelation) Project(removed []int) (*CheckedRelation, error) {
	if e := r.verify("project source"); e != nil {
		return nil, e
	}
	p, e := r.primary.Project(removed)
	if e != nil {
		return nil, e
	}
	s, e := r.checker.Project(removed)
	if e != nil {
		return nil, e
	}
	sig, e := p.Signature()
	if e != nil {
		return nil, e
	}
	out := &CheckedRelation{signature: sig, primary: p, checker: s}
	return out, out.verify("project")
}
func (r *CheckedRelation) Rename(cycle []int) (*CheckedRelation, error) {
	if e := r.verify("rename source"); e != nil {
		return nil, e
	}
	p, e := r.primary.RenameCycle(cycle)
	if e != nil {
		return nil, e
	}
	s, e := r.checker.Rename(cycle)
	if e != nil {
		return nil, e
	}
	sig, e := p.Signature()
	if e != nil {
		return nil, e
	}
	out := &CheckedRelation{signature: sig, primary: p, checker: s}
	return out, out.verify("permutation")
}
func (r *CheckedRelation) SelectEqualAndProject(value Element, column int) (*CheckedRelation, error) {
	if e := r.verify("filter project source"); e != nil {
		return nil, e
	}
	p, e := r.primary.SelectEqualAndProject(value, column)
	if e != nil {
		return nil, e
	}
	s, e := r.checker.SelectEqualAndProject(value, column)
	if e != nil {
		return nil, e
	}
	sig, e := p.Signature()
	if e != nil {
		return nil, e
	}
	out := &CheckedRelation{signature: sig, primary: p, checker: s}
	return out, out.verify("filter project")
}
