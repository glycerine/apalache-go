package api

func (i *apiMPSInvocation) result() (MPSRunResult, error) {
	value, err := i.frontend.Result()
	return MPSRunResult{
		Status: value.Status, Objective: value.Objective,
		Columns: value.Columns, Output: i.output.String(),
	}, err
}
