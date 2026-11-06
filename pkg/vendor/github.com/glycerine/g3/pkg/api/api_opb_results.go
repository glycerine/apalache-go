package api

func (i *apiOPBInvocation) result() (result OPBRunResult, err error) {
	result.Output = i.output.String()
	result.Status, err = i.frontend.Status()
	if err != nil {
		return result, err
	}
	result.Document, err = i.frontend.Document()
	if err != nil {
		return result, err
	}
	result.Summary, err = i.frontend.Summary()
	if err != nil {
		return result, err
	}
	result.Elapsed, err = i.frontend.Elapsed()
	return result, err
}
