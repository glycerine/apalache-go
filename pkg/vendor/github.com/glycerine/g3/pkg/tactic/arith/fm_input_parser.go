package arith

// Source: src/tactic/arith/fm_tactic.cpp:374-411,914-976.
import "github.com/glycerine/g3/pkg/ast"

type FMInputParser struct {
	manager      *ast.ASTManager
	store        *FMConstraintStore
	Family       ast.FamilyID
	Variables    *FMVariableTable
	Expressions  *FMExpressionTable
	RealOnly     bool
	AllowClauses bool
	variableIDs  map[ast.ASTHandle]uint32
	booleanIDs   map[ast.ASTHandle]uint32
	forbidden    map[ast.ASTHandle]bool
	closed       bool
}

func NewFMInputParser(m *ast.ASTManager, store *FMConstraintStore, realOnly, allowClauses bool) (*FMInputParser, error) {
	if m == nil || store == nil || store.manager != m {
		return nil, ast.ErrASTHandle
	}
	if err := store.open(); err != nil {
		return nil, err
	}
	if store.index != nil {
		return nil, ast.ErrASTDomain
	}
	family, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	variables := NewFMVariableTable()
	expressions, err := NewFMExpressionTable(m, variables)
	if err != nil {
		return nil, err
	}
	store.index = variables.Index
	return &FMInputParser{manager: m, store: store, Family: family, Variables: variables, Expressions: expressions, RealOnly: realOnly, AllowClauses: allowClauses, variableIDs: map[ast.ASTHandle]uint32{}, booleanIDs: map[ast.ASTHandle]uint32{}, forbidden: map[ast.ASTHandle]bool{}}, nil
}

func (p *FMInputParser) open() error {
	if p == nil || p.closed || p.manager == nil || p.store == nil {
		return ast.ErrASTHandle
	}
	return nil
}

func (p *FMInputParser) Forbid(term ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	base, ok, err := p.variableTerm(term)
	if err != nil || !ok {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	declaration, err := p.manager.GetDecl(base)
	if err != nil {
		return err
	}
	p.forbidden[declaration] = true
	if v, found := p.variableIDs[base]; found {
		p.Variables.Forbidden[v] = true
		p.Variables.Index.Forbidden[v] = true
	}
	return nil
}
