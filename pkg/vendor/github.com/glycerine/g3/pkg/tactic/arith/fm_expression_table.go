package arith

// Source: src/tactic/arith/fm_tactic.cpp:812-846,914-976.
import "github.com/glycerine/g3/pkg/ast"

type FMExpressionTable struct {
	manager   *ast.ASTManager
	variables *FMVariableTable
	terms     []ast.ASTHandle
	booleans  []ast.ASTHandle
	closed    bool
}

func NewFMExpressionTable(m *ast.ASTManager, variables *FMVariableTable) (*FMExpressionTable, error) {
	if m == nil || variables == nil {
		return nil, ast.ErrASTHandle
	}
	return &FMExpressionTable{manager: m, variables: variables, terms: make([]ast.ASTHandle, len(variables.Integer)), booleans: []ast.ASTHandle{{}}}, nil
}

func (t *FMExpressionTable) BindVariable(v uint32, term ast.ASTHandle) error {
	if t == nil || t.closed || int(v) >= len(t.terms) || term.IsNull() || !t.terms[v].IsNull() {
		return ast.ErrASTDomain
	}
	if err := t.manager.IncRef(term); err != nil {
		return err
	}
	t.terms[v] = term
	return nil
}

func (t *FMExpressionTable) AddBoolean(term ast.ASTHandle) (uint32, error) {
	if t == nil || t.closed || term.IsNull() {
		return 0, ast.ErrASTDomain
	}
	if err := t.manager.IncRef(term); err != nil {
		return 0, err
	}
	t.booleans = append(t.booleans, term)
	return uint32(len(t.booleans) - 1), nil
}

func (t *FMExpressionTable) Variable(v uint32) (ast.ASTHandle, error) {
	if t == nil || t.closed || int(v) >= len(t.terms) || t.terms[v].IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return t.terms[v], nil
}

func (t *FMExpressionTable) Boolean(p uint32) (ast.ASTHandle, error) {
	if t == nil || t.closed || p == 0 || int(p) >= len(t.booleans) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return t.booleans[p], nil
}
