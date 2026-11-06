package fp

// Source: dl_table_relation.cpp universal_target_union_fn. An extensional
// finite relation can feed a DOC head without a mixed join: the source compiler
// forwards its tuples through the table plugin's universal target union.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"github.com/glycerine/g3/pkg/util/memory"
)

type udocFiniteInput struct {
	table     rel.FiniteTable // borrowed from the engine's relation manager
	sorts     []ast.ASTHandle
	auxiliary bool // source query predicates inherit their finite input kind
}

type udocFiniteUnionAttempt struct {
	delta  *rel.TypedUDocRelation
	nested memory.ConstructionFailure
}

func (a *udocFiniteUnionAttempt) close() error {
	if a.nested != nil {
		if err := a.nested.Cleanup(); err != nil {
			return err
		}
		a.nested = nil
	}
	if a.delta != nil {
		if err := a.delta.Close(); err != nil {
			return err
		}
		a.delta = nil
	}
	return nil
}

func (c *datalogUDocContext) clearFiniteUnionCleanup() error {
	for len(c.finiteUnionCleanup) != 0 {
		last := len(c.finiteUnionCleanup) - 1
		if err := c.finiteUnionCleanup[last].close(); err != nil {
			return err
		}
		c.finiteUnionCleanup = c.finiteUnionCleanup[:last]
	}
	return nil
}

func udocFiniteForwardingInput(state *predicateState) bool {
	return state != nil && len(state.sorts) != 0 && state.explicitFiniteRepresentation && state.table != nil && state.external == nil
}

// classifyUDocFiniteForwarding admits one extensional input, distinct variables
// in their original order, and a DOC head with the same arguments. Generated
// finite query heads also permit the permutation required by mk_query. Other
// constants, projections, permutations and mixed joins need more source lowering.
func classifyUDocFiniteForwarding(m *ast.ASTManager, rule *base.Rule, inputs map[ast.ASTHandle]*udocFiniteInput) (head, source ast.ASTHandle, resultErr error) {
	head, err := rule.HeadDeclaration()
	if err != nil {
		return head, source, err
	}
	if input := inputs[head]; input != nil && !input.auxiliary {
		return head, source, ErrUDocDatalogFragment
	}
	total, err := rule.TailSize()
	if err != nil {
		return head, source, err
	}
	uninterpreted, err := rule.UninterpretedTailSize()
	if err != nil {
		return head, source, err
	}
	for i := 0; i < uninterpreted; i++ {
		tail, negative, err := rule.Tail(i)
		if err != nil {
			return head, source, err
		}
		declaration, err := m.GetDecl(tail)
		if err != nil {
			return head, source, err
		}
		input := inputs[declaration]
		if input == nil {
			continue
		}
		if total != 1 || negative {
			return head, source, ErrUDocDatalogFragment
		}
		h, err := rule.Head()
		if err != nil {
			return head, source, err
		}
		hcount, err := m.GetNumArgs(h)
		if err != nil || int(hcount) != len(input.sorts) {
			return head, source, errors.Join(err, ErrUDocDatalogFragment)
		}
		tcount, err := m.GetNumArgs(tail)
		if err != nil || tcount != hcount {
			return head, source, errors.Join(err, ErrUDocDatalogFragment)
		}
		seen := make(map[uint32]bool)
		sourceArguments := make([]ast.ASTHandle, hcount)
		for column := uint32(0); column < hcount; column++ {
			argument, err := m.GetArg(tail, column)
			if err != nil {
				return head, source, err
			}
			kind, err := m.GetKind(argument)
			if err != nil || kind != ast.ASTVar {
				return head, source, errors.Join(err, ErrUDocDatalogFragment)
			}
			index, err := m.GetVarIndex(argument)
			if err != nil || seen[index] {
				return head, source, errors.Join(err, ErrUDocDatalogFragment)
			}
			seen[index] = true
			sourceArguments[column] = argument
			harg, err := m.GetArg(h, column)
			if err != nil || inputs[head] == nil && harg != argument {
				return head, source, errors.Join(err, ErrUDocDatalogFragment)
			}
			sort, err := m.GetSort(argument)
			if err != nil || sort != input.sorts[column] {
				return head, source, errors.Join(err, ErrUDocDatalogFragment)
			}
		}
		if inputs[head] != nil {
			// Source query heads list free variables in descending order, so
			// preserve that finite auxiliary permutation before its table union.
			used := make(map[ast.ASTHandle]bool)
			for column := uint32(0); column < hcount; column++ {
				argument, err := m.GetArg(h, column)
				if err != nil || used[argument] {
					return head, source, errors.Join(err, ErrUDocDatalogFragment)
				}
				found := false
				for _, candidate := range sourceArguments {
					found = found || candidate == argument
				}
				if !found {
					return head, source, ErrUDocDatalogFragment
				}
				used[argument] = true
			}
		}
		source = declaration
	}
	if inputs[head] != nil && source.IsNull() {
		return head, source, ErrUDocDatalogFragment
	}
	return head, source, nil
}

func (e *PropositionalEngine) udocFiniteForwardingOnly(inputs map[ast.ASTHandle]*udocFiniteInput) (bool, error) {
	used := false
	count, err := e.set.Len()
	if err != nil {
		return false, err
	}
	for i := 0; i < count; i++ {
		rule, err := e.set.Rule(i)
		if err != nil {
			return false, err
		}
		_, source, cause := classifyUDocFiniteForwarding(e.manager, rule, inputs)
		used = used || !source.IsNull()
		closeErr := rule.Close()
		if cause != nil || closeErr != nil {
			if errors.Is(cause, ErrUDocDatalogFragment) && closeErr == nil {
				return false, nil
			}
			return false, errors.Join(cause, closeErr)
		}
	}
	// Before any DOC adoption, facts alone do not establish that this mixed
	// program uses the forwarding slice. Keep its existing finite route until
	// a supported rule supplies the live consumer.
	return used || e.udocDatalog != nil, nil
}

func (c *datalogUDocContext) evaluateFiniteForwarding(rule *base.Rule, head, source ast.ASTHandle) (changed bool, resultErr error) {
	target, table := c.result[head], c.finiteResult[source]
	if table == nil {
		return false, ErrUDocDatalogFragment
	}
	if output := c.finiteResult[head]; output != nil {
		// Query auxiliaries preserve the source table kind. Their identity
		// or permutation forwarding uses table rename and ordinary union.
		h, err := rule.Head()
		if err != nil {
			return false, err
		}
		tail, _, err := rule.Tail(0)
		if err != nil {
			return false, err
		}
		columns := make([]int, len(c.finiteInputs[head].sorts))
		for i := range columns {
			argument, err := c.manager.GetArg(h, uint32(i))
			if err != nil {
				return false, err
			}
			columns[i] = -1
			for j := range columns {
				candidate, err := c.manager.GetArg(tail, uint32(j))
				if err != nil {
					return false, err
				}
				if candidate == argument {
					columns[i] = j
					break
				}
			}
			if columns[i] < 0 {
				return false, ErrUDocDatalogFragment
			}
		}
		facts, err := table.Facts()
		if err != nil {
			return false, err
		}
		for _, fact := range facts {
			ordered := make(rel.Fact, len(columns))
			for i, column := range columns {
				ordered[i] = fact[column]
			}
			added, err := output.Add(ordered)
			if err != nil {
				return false, err
			}
			changed = changed || added
		}
		return changed, nil
	}
	if target == nil {
		return false, ErrUDocDatalogFragment
	}
	attempt := &udocFiniteUnionAttempt{}
	c.finiteUnionCleanup = append(c.finiteUnionCleanup, attempt)
	defer func() {
		errors.As(resultErr, &attempt.nested)
		resultErr = errors.Join(resultErr, c.clearFiniteUnionCleanup())
	}()
	delta, err := c.factory.NewEmpty(target.Sorts())
	attempt.delta = delta
	if err != nil {
		return false, err
	}
	if err = rel.UnionUDocFromTable(target, table, c.relations, delta); err != nil {
		return false, err
	}
	empty, err := delta.Empty()
	return !empty, err
}

// A finite input keeps its own table formula and storage; its DOC consumers
// obtain facts through universal union, never through a mirrored answer table.
func (c *datalogUDocContext) finiteInputAnswer(declaration ast.ASTHandle, table rel.FiniteTable) (result ast.ASTRef, resultErr error) {
	input := c.finiteInputs[declaration]
	if input == nil {
		return result, ErrUDocDatalogFragment
	}
	attempt := &storedFactFormulaAttempt{manager: c.manager}
	defer func() {
		if resultErr != nil {
			if result != (ast.ASTRef{}) {
				attempt.outputs = append(attempt.outputs, result)
				result = ast.ASTRef{}
			}
			resultErr = attempt.fail(resultErr)
		}
	}()
	facts, err := table.Facts()
	if err != nil {
		return result, err
	}
	rows := make([]ast.ASTHandle, 0, len(facts))
	for _, fact := range facts {
		values, err := c.relations.DecodeFact(input.sorts, fact)
		attempt.current = values
		if err != nil {
			return result, err
		}
		equalities := make([]ast.ASTHandle, len(values))
		for i, ref := range values {
			value, err := c.manager.RefGet(ref)
			if err != nil {
				return result, err
			}
			root, err := c.manager.NewRef(ast.ASTHandle{})
			if root != (ast.ASTRef{}) {
				attempt.current = append(attempt.current, root)
			}
			if err != nil {
				return result, err
			}
			variable, err := c.manager.MkVar(uint32(i), input.sorts[i])
			if err = errors.Join(err, c.manager.SetRef(root, variable)); err != nil {
				return result, err
			}
			root, err = c.manager.NewRef(ast.ASTHandle{})
			if root != (ast.ASTRef{}) {
				attempt.current = append(attempt.current, root)
			}
			if err != nil {
				return result, err
			}
			equalities[i], err = c.manager.MkEq(variable, value)
			if err = errors.Join(err, c.manager.SetRef(root, equalities[i])); err != nil {
				return result, err
			}
		}
		root, err := c.manager.NewRef(ast.ASTHandle{})
		if root != (ast.ASTRef{}) {
			attempt.outputs = append(attempt.outputs, root)
		}
		if err != nil {
			return result, err
		}
		row, err := c.manager.MkAndArray(equalities)
		if err = errors.Join(err, c.manager.SetRef(root, row)); err != nil {
			return result, err
		}
		rows = append(rows, row)
		if err = attempt.clearCurrent(); err != nil {
			return result, err
		}
	}
	result, err = c.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	formula, err := c.manager.MkOrArray(rows)
	if err = errors.Join(err, c.manager.SetRef(result, formula)); err != nil {
		return result, err
	}
	return result, attempt.close()
}
