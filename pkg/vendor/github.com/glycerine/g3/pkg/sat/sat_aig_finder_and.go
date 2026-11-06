package sat

func (f *satAIGFinder) findAND(c []Literal) (bool, error) {
	if len(c) <= 2 || f.onAIG == nil {
		return false, nil
	}
	for _, head := range c {
		ok := true
		for _, tail := range c {
			if head != tail && !f.implies(head, tail.Negated()) {
				ok = false
				break
			}
		}
		if ok {
			ands := make([]Literal, 0, len(c)-1)
			for _, tail := range c {
				if tail != head {
					ands = append(ands, tail.Negated())
				}
			}
			if err := f.onAIG(head, ands); err != nil {
				return false, err
			}
			return true, nil
		}
	}
	return false, nil
}
