package smtcore

func (e *smtImpliedEqualityEngine) exact(indices []uint32) error {
	for position := 1; position < len(indices); position++ {
		current := indices[position]
		if e.classes.find(current) != current {
			continue
		}
		for priorPosition := 0; priorPosition < position; priorPosition++ {
			prior := indices[priorPosition]
			equal, err := e.implied(current, prior)
			if err != nil {
				return err
			}
			if equal {
				e.classes.merge(current, prior)
				break
			}
		}
	}
	return nil
}
