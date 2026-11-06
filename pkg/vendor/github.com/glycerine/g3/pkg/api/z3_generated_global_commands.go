package api

// Source: generated src/api/api_commands.cpp, global parameter command IDs
// emitted by scripts/update_api.py for the checked-in API headers.

// RegisterZ3GeneratedGlobalCommands installs the generated global-parameter
// replay handlers. The caller owns the replayer; other API handlers are not
// installed by this partial registration.
func RegisterZ3GeneratedGlobalCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	if err := r.RegisterCommand(0, "Z3_global_param_set", replayGlobalParamSet); err != nil {
		return err
	}
	if err := r.RegisterCommand(1, "Z3_global_param_reset_all", replayGlobalParamReset); err != nil {
		return err
	}
	return r.RegisterCommand(2, "Z3_global_param_get", replayGlobalParamGet)
}

func replayGlobalParamSet(r *Z3Replayer) error {
	name, err := r.String(0)
	if err != nil {
		return err
	}
	value, err := r.String(1)
	if err != nil {
		return err
	}
	return GlobalParamSet(name, value)
}
func replayGlobalParamReset(r *Z3Replayer) error { return GlobalParamResetAll() }
func replayGlobalParamGet(r *Z3Replayer) error {
	name, err := r.String(0)
	if err != nil {
		return err
	}
	if _, err = r.String(1); err != nil {
		return err
	}
	if err = r.setStringNull(1); err != nil {
		return err
	}
	value, ok, err := GlobalParamGet(name)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	return r.SetString(1, value)
}
