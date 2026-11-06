package opt

// Source: src/opt/sortmax.cpp sorting-network callback owner.

type sortMaxEncoder struct {
	engine *sortMaxEngine
}

func newSortMaxEncoder(engine *sortMaxEngine) *sortMaxEncoder {
	return &sortMaxEncoder{engine: engine}
}
