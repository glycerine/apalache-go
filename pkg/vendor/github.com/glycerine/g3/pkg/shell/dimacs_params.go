package shell

// Source: src/shell/dimacs_frontend.cpp:96,112. The source reads the current
// process SAT module after command-line assignments have been applied.
func (f *DIMACSFrontend) coreEnabled() (bool, error) {
	value, err := f.globals.SourceGetValueName("sat.dimacs.core")
	if err != nil {
		return false, err
	}
	return value == "true", nil
}
