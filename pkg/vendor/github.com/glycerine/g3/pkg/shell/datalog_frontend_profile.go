package shell

// Profile returns the copied rule/relation summary observed after the single
// saturation run. It does not trigger another fixedpoint query.
func (f *DatalogFrontend) Profile() (DatalogFrontendProfile, error) {
	if f == nil || !f.saturated {
		return DatalogFrontendProfile{}, ErrDatalogFrontend
	}
	return f.context.FixedpointDatalogProfile()
}
