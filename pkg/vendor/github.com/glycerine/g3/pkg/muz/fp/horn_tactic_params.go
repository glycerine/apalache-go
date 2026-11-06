package fp

// Source: horn_tactic::updt_params and collect_param_descrs.

import uparams "github.com/glycerine/g3/pkg/util/params"

func (t *HornTactic) UpdateParams(params *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if params == nil {
		return t.context.Reset(&t.params)
	}
	return t.context.Assign(&t.params, params)
}

func (t *HornTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if out == nil {
		return tacticOwnerError("nil parameter descriptions")
	}
	source, err := t.globals.SourceModuleDescriptorsName("fixedpoint")
	if err != nil {
		return err
	}
	return out.Copy(source)
}
