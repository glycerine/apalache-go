package sat

func (a *satAIGCuts) FindAndAddLUTs(clauses []satLUTClause, binaries []satLUTBinary) ([]satLUTClause, error) {
	var callbackErr error
	f := newSATLUTFinder(func(table uint64, args []BoolVar, out BoolVar) {
		if callbackErr == nil {
			callbackErr = a.AddLUT(uint32(out), table, args)
		}
	})
	remaining, e := f.Run(clauses, binaries)
	if e != nil {
		return nil, e
	}
	if callbackErr != nil {
		return nil, callbackErr
	}
	return remaining, nil
}
