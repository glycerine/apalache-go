package api

// Source: src/api/api_interp.cpp Z3_read_interpolation_problem.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
)

// InterpolationProblem is a parsed sequence or tree interpolation problem.
// Its AST handles are borrowed from the reading InterpolationContext until
// that context is closed. Parents is nil for a sequence; tree parents use the
// interp.ScopeTop as the root sentinel, matching the source API.
type InterpolationProblem struct {
	Constraints []ast.ASTHandle
	Parents     []int
	Theory      []ast.ASTHandle
}

// ReadInterpolationProblem parses the retained interpolation file format.
// The optional first-line comment `;! THEORY=n` marks the leading n
// assertions as background theory. In tree mode, each remaining assertion
// encodes child markers and a local constraint as nested implications.
func (c *InterpolationContext) ReadInterpolationProblem(input io.Reader, tree bool) (result InterpolationProblem, resultErr error) {
	if err := c.open(); err != nil {
		return result, err
	}
	if input == nil {
		return result, ast.ErrASTHandle
	}
	reader := bufio.NewReader(input)
	firstLine, readErr := reader.ReadString('\n')
	if readErr != nil && readErr != io.EOF {
		return result, readErr
	}
	if readErr == io.EOF && firstLine == "" {
		return result, nil
	}
	theoryCount, err := interpolationTheoryCount(firstLine)
	if err != nil {
		return result, err
	}
	parseContext, err := NewSMTContext()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, parseContext.Close()) }()
	parsed, err := parseContext.ParseSMT2(io.MultiReader(strings.NewReader(firstLine), reader), SMT2Bindings{})
	if err != nil {
		return result, err
	}
	sourceManager, err := parseContext.Manager()
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, sourceManager.DelRef(&parsed)) }()
	combined, err := sourceManager.RefGet(parsed)
	if err != nil {
		return result, err
	}
	assertions, err := interpolationConjuncts(sourceManager, combined)
	if err != nil {
		return result, err
	}
	if theoryCount > uint32(len(assertions)) {
		theoryCount = uint32(len(assertions))
	}
	translated, refs, err := c.translateInterpolationAssertions(sourceManager, assertions)
	if err != nil {
		return result, err
	}
	defer func() {
		if resultErr != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
			}
		}
	}()
	theoryCountInt := int(theoryCount)
	result.Theory = append([]ast.ASTHandle(nil), translated[:theoryCountInt]...)
	formulas := translated[theoryCountInt:]
	if tree {
		result.Constraints, result.Parents, err = readInterpolationTree(c.manager, formulas)
		if err != nil {
			return InterpolationProblem{}, err
		}
	} else {
		result.Constraints = append([]ast.ASTHandle(nil), formulas...)
	}
	c.retained = append(c.retained, refs...)
	return result, nil
}

func interpolationTheoryCount(line string) (uint32, error) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, ";!") {
		return 0, nil
	}
	for _, token := range strings.Fields(strings.TrimSpace(strings.TrimPrefix(line, ";!"))) {
		key, value, found := strings.Cut(token, "=")
		if !found || key != "THEORY" {
			continue
		}
		count, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return 0, fmt.Errorf("interpolation: invalid THEORY count %q: %w", value, err)
		}
		return uint32(count), nil
	}
	return 0, nil
}

func interpolationConjuncts(manager *ast.ASTManager, expression ast.ASTHandle) ([]ast.ASTHandle, error) {
	kind, err := manager.GetDeclKind(expression)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTOpTrue {
		return nil, nil
	}
	if kind != ast.ASTOpAnd {
		return []ast.ASTHandle{expression}, nil
	}
	count, err := manager.GetNumArgs(expression)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, count)
	for i := range result {
		result[i], err = manager.GetArg(expression, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (c *InterpolationContext) translateInterpolationAssertions(source *ast.ASTManager, assertions []ast.ASTHandle) (result []ast.ASTHandle, refs []ast.ASTRef, resultErr error) {
	translation, err := ast.NewASTTranslation(source, c.manager, true)
	if err != nil {
		return nil, nil, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, translation.Close())
		if resultErr != nil {
			for i := len(refs) - 1; i >= 0; i-- {
				resultErr = errors.Join(resultErr, c.manager.DelRef(&refs[i]))
			}
			result = nil
			refs = nil
		}
	}()
	translated := make([]ast.ASTHandle, 0, len(assertions))
	refs = make([]ast.ASTRef, 0, len(assertions))
	for _, assertion := range assertions {
		handle, translateErr := translation.Translate(assertion)
		if translateErr != nil {
			return nil, refs, translateErr
		}
		ref, refErr := c.manager.NewRef(handle)
		if refErr != nil {
			return nil, refs, refErr
		}
		translated = append(translated, handle)
		refs = append(refs, ref)
	}
	result = translated
	return result, refs, nil
}

func readInterpolationTree(manager *ast.ASTManager, formulas []ast.ASTHandle) ([]ast.ASTHandle, []int, error) {
	if len(formulas) == 0 {
		return nil, nil, ast.ErrASTDomain
	}
	constraints := make([]ast.ASTHandle, len(formulas))
	parents := make([]int, len(formulas))
	for i := range parents {
		parents[i] = -1
	}
	names := make(map[ast.ASTHandle]int, len(formulas))
	for index, formula := range formulas {
		chain, name, err := interpolationTreeChain(manager, formula)
		if err != nil {
			return nil, nil, fmt.Errorf("interpolation: formula %d: %w", index+1, err)
		}
		if len(chain) == 0 {
			return nil, nil, fmt.Errorf("interpolation: formula %d: expected (implies {children} formula parent)", index+1)
		}
		for _, child := range chain[:len(chain)-1] {
			childIndex, found := names[child]
			if !found {
				return nil, nil, fmt.Errorf("interpolation: formula %d: unknown child marker", index+1)
			}
			if parents[childIndex] != -1 {
				return nil, nil, fmt.Errorf("interpolation: formula %d: child marker has multiple parents", index+1)
			}
			parents[childIndex] = index
		}
		local := chain[len(chain)-1]
		if _, duplicate := names[name]; duplicate {
			return nil, nil, fmt.Errorf("interpolation: formula %d: duplicate node marker", index+1)
		}
		constraints[index] = local
		names[name] = index
	}
	for i := 0; i < len(parents)-1; i++ {
		if parents[i] == -1 {
			return nil, nil, fmt.Errorf("interpolation: formula %d: unreferenced node", i+1)
		}
	}
	parents[len(parents)-1] = interp.ScopeTop
	return constraints, parents, nil
}

func interpolationTreeChain(manager *ast.ASTManager, formula ast.ASTHandle) ([]ast.ASTHandle, ast.ASTHandle, error) {
	var antecedents []ast.ASTHandle
	current := formula
	for {
		kind, err := manager.GetDeclKind(current)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		if kind != ast.ASTOpImplies {
			break
		}
		left, err := manager.GetArg(current, 0)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		right, err := manager.GetArg(current, 1)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		leftKind, err := manager.GetDeclKind(left)
		if err != nil {
			return nil, ast.ASTHandle{}, err
		}
		if len(antecedents) == 0 && leftKind == ast.ASTOpAnd {
			parts, splitErr := interpolationConjuncts(manager, left)
			if splitErr != nil {
				return nil, ast.ASTHandle{}, splitErr
			}
			antecedents = append(antecedents, parts...)
		} else {
			antecedents = append(antecedents, left)
		}
		current = right
	}
	return antecedents, current, nil
}
