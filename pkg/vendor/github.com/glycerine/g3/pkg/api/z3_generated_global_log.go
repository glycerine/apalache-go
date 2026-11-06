package api

// Source: generated src/api/api_log_macros.cpp global-parameter records.

func (log *Z3LogWriter) globalParamSet(name, value string) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.String(name); err != nil {
		return err
	}
	if err := log.String(value); err != nil {
		return err
	}
	return log.Call(0)
}
func (log *Z3LogWriter) globalParamReset() error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	return log.Call(1)
}
func (log *Z3LogWriter) globalParamGet(name string) error {
	if err := log.ResetArguments(); err != nil {
		return err
	}
	if err := log.String(name); err != nil {
		return err
	}
	if err := log.String(""); err != nil {
		return err
	}
	return log.Call(2)
}
