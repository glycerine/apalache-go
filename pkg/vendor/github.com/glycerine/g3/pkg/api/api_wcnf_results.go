package api

func (i *apiWCNFInvocation) result() (r WCNFRunResult, e error) {
	r.Output = i.output.String()
	r.Status, e = i.frontend.Status()
	if e != nil {
		return
	}
	r.Document, e = i.frontend.Document()
	if e != nil {
		return
	}
	r.Summary, e = i.frontend.Summary()
	if e != nil {
		return
	}
	r.Elapsed, e = i.frontend.Elapsed()
	return
}
