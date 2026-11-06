package transforms

// Source: src/muz/transforms/dl_mk_slice.cpp, slice_proof_converter's
// original-rule map and asserted-proof translation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// ErrSliceProofUnsupported indicates a proof shape the converter cannot yet
// translate, allowing a caller to retry with unsliced rules.
var ErrSliceProofUnsupported = errors.New("slice proof conversion unsupported")

// SliceProofConverter owns the rule and formula mappings created by slicing.
// ConvertAsserted handles assertion leaves; Convert also handles supported
// hyper-resolution nodes, coalesce derivations and configured normalization
// wrappers, including quantified conclusions.
type SliceProofConverter struct {
	manager              *ast.ASTManager
	rules                *base.RuleManager
	originals            []*base.Rule
	formulas             []ast.ASTRef
	byFormula            map[ast.ASTHandle]sliceOriginalRule
	predicates           map[ast.ASTHandle]slicePredicateOrigin
	closed               bool
	normalizationParams  *uparams.ParamsContext
	normalizationGlobals *uparams.GlobalParamsService
	normalization        *RuleNormalizationParams
}

type slicePredicateOrigin struct {
	declaration ast.ASTHandle
	removed     []bool
}

type sliceOriginalRule struct {
	rule      *base.Rule
	variables []uint32
}

func newSliceProofConverter(manager *ast.ASTManager, rules *base.RuleManager, predicates map[ast.ASTHandle]ast.ASTHandle, slices map[ast.ASTHandle][]bool) *SliceProofConverter {
	c := &SliceProofConverter{manager: manager, rules: rules, byFormula: make(map[ast.ASTHandle]sliceOriginalRule), predicates: make(map[ast.ASTHandle]slicePredicateOrigin)}
	for original, derived := range predicates {
		c.predicates[derived] = slicePredicateOrigin{declaration: original, removed: append([]bool(nil), slices[original]...)}
	}
	return c
}

func (c *SliceProofConverter) insert(original, sliced *base.Rule, variables []uint32) (resultErr error) {
	if c == nil || c.manager == nil || original == nil || sliced == nil {
		return ast.ErrASTHandle
	}
	formula, err := sliced.ToFormula()
	if err != nil {
		return err
	}
	defer func() {
		if formula != (ast.ASTRef{}) {
			resultErr = errors.Join(resultErr, c.manager.DelRef(&formula))
		}
	}()
	fact, err := c.manager.RefGet(formula)
	if err != nil {
		return err
	}
	owner, err := original.Retain()
	if err != nil {
		return err
	}
	c.originals = append(c.originals, owner)
	c.formulas = append(c.formulas, formula)
	formula = ast.ASTRef{}
	c.byFormula[fact] = sliceOriginalRule{rule: owner, variables: append([]uint32(nil), variables...)}
	return nil
}

// OriginalVariables maps each sliced rule variable index to its original index.
// The returned slice is independent of the converter's internal mapping.
func (c *SliceProofConverter) OriginalVariables(proof ast.ASTHandle) ([]uint32, bool, error) {
	if c == nil || c.manager == nil {
		return nil, false, ast.ErrASTHandle
	}
	var fact ast.ASTHandle
	matched, err := c.manager.MatchProofAsserted(proof, &fact)
	if err != nil || !matched {
		return nil, false, err
	}
	entry, found := c.byFormula[fact]
	if !found {
		return nil, false, nil
	}
	return append([]uint32(nil), entry.variables...), true, nil
}

// ConvertAsserted maps an asserted sliced rule to the original rule's proof.
// The returned reference is owned by the caller when matched is true.
func (c *SliceProofConverter) ConvertAsserted(proof ast.ASTHandle) (result ast.ASTRef, matched bool, err error) {
	if c == nil || c.manager == nil {
		return result, false, ast.ErrASTHandle
	}
	var fact ast.ASTHandle
	matched, err = c.manager.MatchProofAsserted(proof, &fact)
	if err != nil || !matched {
		return result, false, err
	}
	entry, found := c.byFormula[fact]
	if !found {
		return result, false, nil
	}
	original, found, err := entry.rule.Proof()
	if err != nil {
		return result, false, err
	}
	if !found {
		return result, false, ast.ErrASTDomain
	}
	result, err = c.manager.NewRef(original)
	return result, err == nil, err
}

func (c *SliceProofConverter) Close() error {
	if c == nil {
		return ast.ErrASTHandle
	}
	if c.closed {
		return nil
	}
	if c.manager == nil {
		return ast.ErrASTHandle
	}
	// A native numerical invocation can call error transport while the AST
	// manager is executing. Reject reentrant destruction before changing owners.
	if _, err := c.manager.ProofMode(); err != nil {
		return err
	}
	for len(c.formulas) != 0 {
		last := len(c.formulas) - 1
		err := c.manager.DelRef(&c.formulas[last])
		if c.formulas[last] == (ast.ASTRef{}) {
			c.formulas = c.formulas[:last]
		}
		if err != nil {
			return err
		}
	}
	for len(c.originals) != 0 {
		last := len(c.originals) - 1
		if err := c.originals[last].Close(); err != nil {
			return err
		}
		c.originals[last] = nil
		c.originals = c.originals[:last]
	}
	c.manager = nil
	c.rules = nil
	c.byFormula = nil
	c.predicates = nil
	c.normalizationParams = nil
	c.normalizationGlobals = nil
	c.normalization = nil
	c.closed = true
	return nil
}

// SetNormalizationServices enables replay of live rule-normalization wrappers
// after reconstructing their parent rules in the original predicate vocabulary.
func (c *SliceProofConverter) SetNormalizationServices(context *uparams.ParamsContext, globals *uparams.GlobalParamsService) error {
	if c == nil || c.manager == nil || context == nil || globals == nil || globals.Context != context || context.SymbolsField != c.manager.Symbols {
		return ast.ErrASTHandle
	}
	c.normalizationParams = context
	c.normalizationGlobals = globals
	return nil
}

// SetNormalizationParams copies the query's rule-normalization switches for
// proof reconstruction after its parameter view has closed.
func (c *SliceProofConverter) SetNormalizationParams(params RuleNormalizationParams) {
	c.normalization = &params
}
