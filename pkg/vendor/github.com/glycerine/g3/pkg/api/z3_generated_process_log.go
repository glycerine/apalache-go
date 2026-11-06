package api

// Source: generated src/api/api_log_macros.cpp, process and version records.

func (log *Z3LogWriter) logToggleWarnings(enabled bool) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	value := int64(0)
	if enabled {
		value = 1
	}
	if err := log.Int64(value); err != nil {
		return err
	}
	return log.Call(327)
}
func (log *Z3LogWriter) logVersion() error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	for i := 0; i < 4; i++ {
		if err := log.Uint64(0); err != nil {
			return err
		}
	}
	return log.Call(342)
}
func (log *Z3LogWriter) logFullVersion() error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	return log.Call(343)
}
func (log *Z3LogWriter) logTrace(tag string, enable bool) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.String(tag); err != nil {
		return err
	}
	if enable {
		return log.Call(344)
	}
	return log.Call(345)
}
