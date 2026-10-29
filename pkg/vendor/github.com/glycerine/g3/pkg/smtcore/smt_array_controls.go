package smtcore

// Source: theory_array::instantiate_extensionality and theory_array_params.

import "github.com/glycerine/g3/pkg/ast"

func (t *SMTArrayTheory) ConfigureArray(config SMTArrayState) error {
	if t == nil {
		return ast.ErrASTHandle
	}
	if config.LazyIEq && config.LazyIEqDelay == 0 {
		return ast.ErrASTDomain
	}
	t.extensional = config.Extensional
	t.arrayConfig = config
	return nil
}

func (k *SMTKernel) configureArrayTheories(config SMTArrayState) error {
	if k.copiedArrayOwner != nil {
		config = k.copiedArrayOwner.state
	}
	for _, theory := range k.internalizer.theoryOrder {
		if array := smtArrayCore(theory); array != nil {
			if err := array.ConfigureArray(config); err != nil {
				return err
			}
		}
	}
	return nil
}
