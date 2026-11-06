package api

// Source: generated src/api/api_log_macros.cpp, config command records.
// The caller emits Result after a successful config creation, matching the
// source RETURN_Z3 macro's separate result record.

func (log *Z3LogWriter) LogZ3MkConfig() error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	return log.Call(3)
}
func (log *Z3LogWriter) LogZ3DelConfig(pointer uint64) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.Pointer(pointer); err != nil {
		return err
	}
	return log.Call(4)
}
func (log *Z3LogWriter) LogZ3SetParamValue(pointer uint64, name, value string) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.Pointer(pointer); err != nil {
		return err
	}
	if err := log.String(name); err != nil {
		return err
	}
	if err := log.String(value); err != nil {
		return err
	}
	return log.Call(5)
}

// configID provides stable log-local identities for concrete Go configs.
func (log *Z3LogWriter) configID(config *Config) (uint64, error) {
	if log == nil || log.closed || config == nil {
		return 0, ErrZ3LogClosed
	}
	if id := log.configIDs[config]; id != 0 {
		return id, nil
	}
	if log.nextObjectID == ^uint64(0) {
		return 0, ErrZ3LogWriter
	}
	if log.configIDs == nil {
		log.configIDs = make(map[*Config]uint64)
	}
	log.nextObjectID++
	log.configIDs[config] = log.nextObjectID
	return log.nextObjectID, nil
}
