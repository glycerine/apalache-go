package api

// Source: generated src/api/api_commands.cpp, config commands 3 through 5.

// RegisterZ3GeneratedConfigCommands installs the generated config replay
// handlers. The replayer owns any configurations left open by the log.
func RegisterZ3GeneratedConfigCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	if err := r.RegisterCommand(3, "Z3_mk_config", replayMkConfig); err != nil {
		return err
	}
	if err := r.RegisterCommand(4, "Z3_del_config", replayDelConfig); err != nil {
		return err
	}
	return r.RegisterCommand(5, "Z3_set_param_value", replaySetConfigParam)
}

func replayMkConfig(r *Z3Replayer) error {
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	config, err := MkConfig()
	if err != nil {
		return err
	}
	if r.configs == nil {
		r.configs = make(map[uint64]*Config)
	}
	r.nextObjectID++
	id := r.nextObjectID
	r.configs[id] = config
	r.configOrder = append(r.configOrder, id)
	return r.StoreResult(id)
}

func replayConfig(r *Z3Replayer, position uint32) (*Config, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	config := r.configs[id]
	if config == nil {
		return nil, ErrZ3ReplayReference
	}
	return config, nil
}

func replayDelConfig(r *Z3Replayer) error {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	config := r.configs[id]
	if config == nil {
		return ErrZ3ReplayReference
	}
	if err = config.Close(); err != nil {
		return err
	}
	delete(r.configs, id)
	return nil
}

func replaySetConfigParam(r *Z3Replayer) error {
	config, err := replayConfig(r, 0)
	if err != nil {
		return err
	}
	name, err := r.String(1)
	if err != nil {
		return err
	}
	value, err := r.String(2)
	if err != nil {
		return err
	}
	return SetParamValue(config, name, value)
}
