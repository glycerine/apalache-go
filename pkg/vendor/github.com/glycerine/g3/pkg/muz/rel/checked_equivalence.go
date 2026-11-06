package rel

func factSet(facts []Fact) map[string]bool {
	r := make(map[string]bool, len(facts))
	for _, f := range facts {
		r[factKey(f)] = true
	}
	return r
}
func (r *CheckedRelation) verify(objective string) error {
	if e := r.open(); e != nil {
		return e
	}
	a, e := r.primary.Facts()
	if e != nil {
		return e
	}
	b, e := r.checker.Facts()
	if e != nil {
		return e
	}
	if len(a) != len(b) {
		return r.fail(objective)
	}
	set := factSet(a)
	for _, f := range b {
		if !set[factKey(f)] {
			return r.fail(objective)
		}
	}
	return nil
}
func (r *CheckedRelation) Consistent() error { return r.verify("consistent formula") }
