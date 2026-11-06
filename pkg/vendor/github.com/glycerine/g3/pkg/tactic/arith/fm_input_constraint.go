package arith

// Source: src/tactic/arith/fm_tactic.cpp:490-510,997-1086.
import "github.com/glycerine/g3/pkg/ast"

func (p *FMInputParser) Parse(formula ast.ASTHandle, dependency ast.ASTDependency) (*FMConstraint, FMRegistration, bool, error) {
	if err := p.open(); err != nil {
		return nil, FMRegistrationDeferred, false, err
	}
	occ, err := p.isOCC(formula)
	if err != nil || !occ {
		return nil, FMRegistrationDeferred, false, err
	}
	args := []ast.ASTHandle{formula}
	if p.AllowClauses {
		kind, err := p.manager.GetKind(formula)
		if err != nil {
			return nil, FMRegistrationDeferred, false, err
		}
		if kind == ast.ASTApp {
			family, err := p.manager.GetFamilyID(formula)
			if err != nil {
				return nil, FMRegistrationDeferred, false, err
			}
			op, err := p.manager.GetDeclKind(formula)
			if err != nil {
				return nil, FMRegistrationDeferred, false, err
			}
			if family == p.manager.BasicFamilyID() && op == ast.ASTOpOr {
				n, err := p.manager.GetNumArgs(formula)
				if err != nil {
					return nil, FMRegistrationDeferred, false, err
				}
				args = args[:0]
				for i := uint32(0); i < n; i++ {
					h, err := p.manager.GetArg(formula, i)
					if err != nil {
						return nil, FMRegistrationDeferred, false, err
					}
					args = append(args, h)
				}
			}
		}
	}
	inequalityIndex := -1
	for i, arg := range args {
		if _, _, literal, err := p.literal(arg); err != nil {
			return nil, FMRegistrationDeferred, false, err
		} else if literal {
			continue
		}
		if inequalityIndex >= 0 {
			return nil, FMRegistrationDeferred, false, ast.ErrASTDomain
		}
		inequalityIndex = i
	}
	if inequalityIndex < 0 {
		return nil, FMRegistrationDeferred, false, nil
	}
	literals := make([]int32, 0, len(args)-1)
	for i, arg := range args {
		if i == inequalityIndex {
			continue
		}
		base, negative, _, err := p.literal(arg)
		if err != nil {
			return nil, FMRegistrationDeferred, false, err
		}
		id, err := p.internBoolean(base)
		if err != nil {
			return nil, FMRegistrationDeferred, false, err
		}
		if id > uint32(^uint32(0)>>1) {
			return nil, FMRegistrationDeferred, false, ast.ErrASTDomain
		}
		literal := int32(id)
		if negative {
			literal = -literal
		}
		literals = append(literals, literal)
	}
	xs, coefficients, constant, strict, _, err := p.inequality(args[inequalityIndex], true)
	if err != nil {
		return nil, FMRegistrationDeferred, false, err
	}
	if err = p.store.diagnostics.inputVariables(xs); err != nil {
		return nil, FMRegistrationDeferred, false, err
	}
	c, err := FMBuildConstraint(p.store, p.Variables, literals, xs, coefficients, &constant, strict, dependency)
	if err != nil {
		return nil, FMRegistrationDeferred, false, err
	}
	if err = p.store.diagnostics.inputConstraint(c); err != nil {
		return c, FMRegistrationDeferred, true, err
	}
	registration := p.Variables.Register(c)
	if registration == FMRegistrationDeferred {
		err = p.store.diagnostics.deferred(c)
	}
	return c, registration, true, err
}
