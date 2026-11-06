package qe

import "github.com/glycerine/g3/pkg/ast"

// Prepare retains update_bounds: only atoms containing x are classified and a
// single unrecognized atom rejects the arithmetic plugin for this formula.
func (p *ArithmeticPlugin) Prepare(variable *ast.ContainsApp, formula ast.ASTHandle, positive, negative []ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if variable == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	key := arithmeticPluginKey{target, formula}
	if p.bounds[key] != nil {
		return true, nil
	}
	processor, err := NewArithmeticBoundsProcessor(p.util)
	if err != nil {
		return false, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = processor.Close()
		}
	}()
	for polarity, atoms := range [][]ast.ASTHandle{positive, negative} {
		for _, atom := range atoms {
			contains, cause := variable.Check(atom)
			if cause != nil {
				return false, cause
			}
			if !contains {
				continue
			}
			normalized := atom
			var owned ast.ASTRef
			if polarity == 1 {
				owned, _, cause = p.util.NormalizeAtom(atom, false)
				if cause != nil {
					return false, cause
				}
				normalized, cause = p.manager.RefGet(owned)
				if cause != nil {
					_ = p.manager.DelRef(&owned)
					return false, cause
				}
			}
			matched, cause := processor.Add(variable, normalized)
			if polarity == 1 {
				cause = joinArithmeticErrors(cause, p.manager.DelRef(&owned))
			}
			if cause != nil {
				return false, cause
			}
			if !matched {
				return false, nil
			}
		}
	}
	if err = p.insertEntry(target, formula, processor); err != nil {
		return false, err
	}
	ok = true
	return true, nil
}

func joinArithmeticErrors(a, b error) error {
	if a != nil {
		return a
	}
	return b
}
