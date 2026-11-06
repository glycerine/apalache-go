package sat

func (a *satAIGCuts) Close() error {
	if err := a.open(); err != nil {
		return err
	}
	for _, cs := range a.cuts {
		if err := cs.Close(); err != nil {
			return err
		}
	}
	a.budget.Release(a.ownerBytes)
	a.nodes, a.literals, a.cuts, a.roots = nil, nil, nil, nil
	a.ownerBytes, a.closed = 0, true
	return nil
}
