package arith

// Source: src/tactic/arith/fm_tactic.cpp:413-445,914-976.
import "github.com/glycerine/g3/pkg/ast"

func (p *FMInputParser) uninterpretedConstant(term ast.ASTHandle) (bool, error) {
	kind, err := p.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	family, err := p.manager.GetFamilyID(term)
	if err != nil || family != ast.NullFamilyID {
		return false, err
	}
	n, err := p.manager.GetNumArgs(term)
	return n == 0, err
}

func (p *FMInputParser) variableTerm(term ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if yes, err := p.uninterpretedConstant(term); err != nil || yes {
		return term, yes, err
	}
	kind, err := p.manager.GetKind(term)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, false, err
	}
	family, err := p.manager.GetFamilyID(term)
	if err != nil || family != p.Family {
		return ast.ASTHandle{}, false, err
	}
	op, err := p.manager.GetDeclKind(term)
	if err != nil || op != 14 {
		return ast.ASTHandle{}, false, err
	}
	child, err := p.manager.GetArg(term, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	yes, err := p.uninterpretedConstant(child)
	return child, yes, err
}

func (p *FMInputParser) integerVariable(term ast.ASTHandle) (bool, error) {
	sort, err := p.manager.GetSort(term)
	if err != nil {
		return false, err
	}
	family, err := p.manager.GetFamilyID(sort)
	if err != nil || family != p.Family {
		return false, err
	}
	kind, err := p.manager.GetDeclKind(sort)
	return kind == 1, err
}

func (p *FMInputParser) internVariable(term ast.ASTHandle) (uint32, error) {
	base, ok, err := p.variableTerm(term)
	if err != nil || !ok {
		if err != nil {
			return 0, err
		}
		return 0, ast.ErrASTDomain
	}
	if v, found := p.variableIDs[base]; found {
		return v, p.store.diagnostics.toVariable(base, v)
	}
	integer, err := p.integerVariable(base)
	if err != nil {
		return 0, err
	}
	declaration, err := p.manager.GetDecl(base)
	if err != nil {
		return 0, err
	}
	v := p.Variables.Add(integer, p.forbidden[declaration] || p.RealOnly && integer)
	p.Expressions.terms = append(p.Expressions.terms, ast.ASTHandle{})
	if err = p.Expressions.BindVariable(v, base); err != nil {
		p.Expressions.terms = p.Expressions.terms[:len(p.Expressions.terms)-1]
		p.Variables.Integer = p.Variables.Integer[:len(p.Variables.Integer)-1]
		p.Variables.Forbidden = p.Variables.Forbidden[:len(p.Variables.Forbidden)-1]
		delete(p.Variables.Index.Forbidden, v)
		return 0, err
	}
	p.variableIDs[base] = v
	return v, p.store.diagnostics.toVariable(base, v)
}

func (p *FMInputParser) internBoolean(term ast.ASTHandle) (uint32, error) {
	if v, found := p.booleanIDs[term]; found {
		return v, nil
	}
	v, err := p.Expressions.AddBoolean(term)
	if err == nil {
		p.booleanIDs[term] = v
	}
	return v, err
}
