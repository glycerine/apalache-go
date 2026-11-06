package sat

import "github.com/glycerine/g3/pkg/arena"

func (a *satAIGCuts) reserve(v uint32) error {
	if err := a.open(); err != nil {
		return err
	}
	if v > uint32(NullBoolVar) {
		return arena.ErrSlotLimit
	}
	start := len(a.nodes)
	for uint32(len(a.nodes)) <= v {
		cs, err := newSATCutSet(a.budget)
		if err != nil {
			for _, made := range a.cuts[start:] {
				_ = made.Close()
			}
			a.nodes, a.cuts = a.nodes[:start], a.cuts[:start]
			a.maxCutSetSize, a.lastTouched = a.maxCutSetSize[:start], a.lastTouched[:start]
			return err
		}
		if err = cs.Init(a.config.maxCutSetSize+1, ^uint32(0)); err != nil {
			_ = cs.Close()
			for _, made := range a.cuts[start:] {
				_ = made.Close()
			}
			a.nodes, a.cuts = a.nodes[:start], a.cuts[:start]
			a.maxCutSetSize, a.lastTouched = a.maxCutSetSize[:start], a.lastTouched[:start]
			return err
		}
		a.nodes = append(a.nodes, nil)
		a.cuts = append(a.cuts, cs)
		a.maxCutSetSize = append(a.maxCutSetSize, a.config.maxCutSetSize)
		a.lastTouched = append(a.lastTouched, 0)
	}
	return nil
}

func (a *satAIGCuts) touch(v uint32) {
	a.lastTouched[v] = v + a.numCutCalls*uint32(len(a.nodes))
}

func (a *satAIGCuts) touched(v uint32) bool {
	return int(v) < len(a.lastTouched) && a.lastTouched[v]+uint32(len(a.nodes)) >= a.numCutCalls*uint32(len(a.nodes))
}

func (a *satAIGCuts) initCutSet(v uint32) error {
	cs := a.cuts[v]
	if err := cs.Reset(a.onCutDel); err != nil {
		return err
	}
	if err := cs.Init(a.config.maxCutSetSize+1, v); err != nil {
		return err
	}
	return cs.PushBack(a.onCutAdd, newSATCut(v))
}
