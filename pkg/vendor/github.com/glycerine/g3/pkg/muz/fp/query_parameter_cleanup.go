package fp

import uparams "github.com/glycerine/g3/pkg/util/params"

// clearQueryParameters keeps the exact stopped params_ref decrement so a
// refused native destruction can be completed without repeating dec_ref.
func (e *PropositionalEngine) clearQueryParameters() error {
	e.queryParamsReady = false
	if e.queryParamsReceipt.Invocation() != 0 {
		if err := e.params.CompleteDecrement(&e.queryParams, e.queryParamsReceipt); err != nil {
			return err
		}
		e.queryParamsReceipt = uparams.ParamsDecrementReceipt{}
		return nil
	}
	if e.queryParams.Slot == 0 {
		return nil
	}
	var receipt uparams.ParamsDecrementReceipt
	if err := e.params.DelTracked(&e.queryParams, &receipt); err != nil {
		if receipt.Invocation() != 0 {
			e.queryParamsReceipt = receipt
		}
		return err
	}
	return nil
}
