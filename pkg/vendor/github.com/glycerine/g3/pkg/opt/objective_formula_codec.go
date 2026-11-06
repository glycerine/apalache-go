package opt

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type objectiveFormulaMetadata struct {
	kind         FormulaObjectiveKind
	originalTerm ast.ASTHandle
	weights      []big.Rat
	id           string
}

type ObjectiveFormulaCodec struct {
	manager      *ast.ASTManager
	declarations []ast.ASTRef
	derived      []ast.ASTRef
	metadata     map[ast.ASTHandle]objectiveFormulaMetadata
	closed       bool
}

func NewObjectiveFormulaCodec(manager *ast.ASTManager) (*ObjectiveFormulaCodec, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := manager.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &ObjectiveFormulaCodec{manager: manager, metadata: make(map[ast.ASTHandle]objectiveFormulaMetadata)}, nil
}

func (c *ObjectiveFormulaCodec) open() error {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return arena.ErrClosed
	}
	return nil
}

func formulaObjectiveName(kind FormulaObjectiveKind) string {
	switch kind {
	case FormulaMaximize:
		return "maximize"
	case FormulaMinimize:
		return "minimize"
	case FormulaMaxSAT:
		return "maxsat"
	default:
		return ""
	}
}

func (c *ObjectiveFormulaCodec) freshDeclaration(kind FormulaObjectiveKind, args []ast.ASTHandle, metadata objectiveFormulaMetadata) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	name := formulaObjectiveName(kind)
	if name == "" {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	domain := make([]ast.ASTHandle, len(args))
	for i, arg := range args {
		sort, err := c.manager.GetSort(arg)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		domain[i] = sort
	}
	boolean, err := c.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	prefix, err := c.manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := c.manager.MkFreshFuncDecl(prefix, usymbol.Symbol{}, domain, boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.manager.NewRef(declaration)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.declarations = append(c.declarations, ref)
	c.metadata[declaration] = metadata
	return declaration, nil
}

func (c *ObjectiveFormulaCodec) Close() (result error) {
	if c == nil || c.manager == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	for i := len(c.declarations) - 1; i >= 0; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.declarations[i]))
	}
	for i := len(c.derived) - 1; i >= 0; i-- {
		result = errors.Join(result, c.manager.DelRef(&c.derived[i]))
	}
	if result == nil {
		c.declarations = nil
		c.derived = nil
		c.metadata = nil
		c.closed = true
	}
	return result
}
