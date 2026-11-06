package api

// Source: generated api_commands.cpp, tactic and probe metadata.

func registerZ3GeneratedTacticRegistry(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{391, "Z3_get_num_tactics"}, {392, "Z3_get_tactic_name"}, {393, "Z3_get_num_probes"}, {394, "Z3_get_probe_name"},
		{395, "Z3_tactic_get_help"}, {396, "Z3_tactic_get_param_descrs"}, {397, "Z3_tactic_get_descr"}, {398, "Z3_probe_get_descr"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayTacticRegistry(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayTacticRegistry(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	switch id {
	case 391:
		_, err = owner.context.TacticCount()
	case 392:
		var index uint32
		index, err = r.Uint(1)
		if err == nil {
			_, err = owner.context.TacticName(index)
		}
	case 393:
		_, err = owner.context.ProbeCount()
	case 394:
		var index uint32
		index, err = r.Uint(1)
		if err == nil {
			_, err = owner.context.ProbeDescriptor(index)
		}
	case 395, 396:
		tactic, e := replayTacticAt(r, owner, 1)
		if e != nil {
			return e
		}
		if id == 395 {
			_, err = tactic.value.Help()
		} else {
			descriptions, e := tactic.value.ParameterDescriptions()
			if e != nil {
				return e
			}
			return storeReplayDescriptions(r, owner, descriptions)
		}
	case 397, 398:
		var name string
		name, err = r.String(1)
		if err == nil {
			if id == 397 {
				_, err = owner.context.TacticDescription(name)
			} else {
				_, err = owner.context.ProbeDescription(name)
			}
		}
	}
	return err
}
