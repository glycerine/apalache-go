package cmd_context

type FixedpointRelationProfile struct {
	Name       string
	TupleCount uint64
}

type FixedpointDatalogProfile struct {
	RuleCount, RelationCount, TupleCount uint64
	Compiled                             bool
	Relations                            []FixedpointRelationProfile
}

func (c *Context) FixedpointDatalogProfile() (FixedpointDatalogProfile, error) {
	engine, err := c.fixedpointContext()
	if err != nil {
		return FixedpointDatalogProfile{}, err
	}
	profile, err := engine.DatalogProfile()
	if err != nil {
		return FixedpointDatalogProfile{}, err
	}
	manager, err := c.Manager()
	if err != nil {
		return FixedpointDatalogProfile{}, err
	}
	result := FixedpointDatalogProfile{RuleCount: profile.RuleCount, RelationCount: profile.RelationCount, TupleCount: profile.TupleCount, Compiled: profile.Compiled, Relations: make([]FixedpointRelationProfile, 0, len(profile.Relations))}
	for _, relation := range profile.Relations {
		symbol, e := manager.GetASTName(relation.Declaration)
		if e != nil {
			return FixedpointDatalogProfile{}, e
		}
		name, e := manager.Symbols.String(symbol)
		if e != nil {
			return FixedpointDatalogProfile{}, e
		}
		result.Relations = append(result.Relations, FixedpointRelationProfile{Name: name, TupleCount: relation.TupleCount})
	}
	return result, nil
}
