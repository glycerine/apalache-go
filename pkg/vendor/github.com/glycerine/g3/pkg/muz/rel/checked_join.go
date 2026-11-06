package rel

func (a *CheckedRelation) Join(b *CheckedRelation, left, right []int) (*CheckedRelation, error) {
	if e := a.verify("join left"); e != nil {
		return nil, e
	}
	if e := b.verify("join right"); e != nil {
		return nil, e
	}
	p, e := a.primary.Join(b.primary, left, right)
	if e != nil {
		return nil, e
	}
	s, e := a.checker.Join(b.checker, left, right)
	if e != nil {
		return nil, e
	}
	sig, e := p.Signature()
	if e != nil {
		return nil, e
	}
	out := &CheckedRelation{signature: sig, primary: p, checker: s}
	return out, out.verify("join")
}
func (a *CheckedRelation) JoinProject(b *CheckedRelation, left, right, removed []int) (*CheckedRelation, error) {
	joined, e := a.Join(b, left, right)
	if e != nil {
		return nil, e
	}
	defer joined.Close()
	return joined.Project(removed)
}
