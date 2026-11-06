package fp

// Concrete retained portion of dl_compiler.cpp's interpreted filter path and
// karr_relation.cpp filter_interpreted over bound integer variables.
import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/rel"
	"math/big"
)

type karrGuardRule struct {
	body           ast.ASTHandle
	condition      ast.ASTHandle
	positions      map[ast.ASTHandle]uint32
	inputConstants []uint32
	inputValues    []ast.ASTHandle
	identical      []uint32
	row            []big.Rat
	constant       big.Rat
	equal          bool
	secondRow      []big.Rat
	secondConstant big.Rat
	projection     []uint32
	head           karrHeadPlan
	more           []karrGuardRule
}

func (a *karrDatalogActivation) compileGuardRule(rule *base.Rule) (ast.ASTHandle, karrGuardRule, error) {
	var none ast.ASTHandle
	var out karrGuardRule
	count, err := rule.TailSize()
	if err != nil {
		return none, out, err
	}
	if count < 2 {
		return none, out, ErrKarrDatalogFragment
	}
	tailCount := count
	count, err = rule.UninterpretedTailSize()
	if err != nil {
		return none, out, err
	}
	if count != 1 {
		return none, out, ErrKarrDatalogFragment
	}
	head, err := rule.Head()
	if err != nil {
		return none, out, err
	}
	tail, neg, err := rule.Tail(0)
	if err != nil {
		return none, out, err
	}
	if neg {
		return none, out, ErrKarrDatalogFragment
	}
	hd, err := a.manager.GetDecl(head)
	if err != nil {
		return none, out, err
	}
	out.body, err = a.manager.GetDecl(tail)
	if err != nil {
		return none, out, err
	}
	if a.seeds[hd] == nil || a.seeds[out.body] == nil {
		return none, out, ErrKarrDatalogFragment
	}
	arity, err := a.manager.GetNumArgs(tail)
	if err != nil {
		return none, out, err
	}
	positions := make(map[ast.ASTHandle]uint32, arity)
	groups := make(map[ast.ASTHandle][]uint32, arity)
	var order []ast.ASTHandle
	columns := make([]ast.ASTHandle, 0, arity)
	for i := uint32(0); i < arity; i++ {
		arg, err := a.manager.GetArg(tail, i)
		if err != nil {
			return none, out, err
		}
		kind, err := a.manager.GetKind(arg)
		if err != nil {
			return none, out, err
		}
		if kind != ast.ASTVar {
			_, _, matched, err := a.manager.ArithmeticNumeralText(arg)
			if err != nil {
				return none, out, err
			}
			if !matched {
				return none, out, ErrKarrDatalogFragment
			}
			// Earlier constants have already been removed from the signature.
			out.inputConstants = append(out.inputConstants, uint32(len(columns)))
			out.inputValues = append(out.inputValues, arg)
			continue
		}
		if len(groups[arg]) == 0 {
			order = append(order, arg)
		}
		column := uint32(len(columns))
		groups[arg] = append(groups[arg], column)
		positions[arg] = column // interpreted bindings use the last column
		columns = append(columns, arg)
	}
	for _, arg := range order {
		if len(groups[arg]) > 1 {
			// Multiple groups depend on the native source map's traversal order.
			if out.identical != nil {
				return none, out, ErrKarrDatalogFragment
			}
			out.identical = groups[arg]
		}
	}
	// The native combined filter/project removes every tail variable absent
	// from the head, sorting source column indices before projection.
	used := make(map[ast.ASTHandle]bool)
	n, err := a.manager.GetNumArgs(head)
	if err != nil {
		return none, out, err
	}
	for i := uint32(0); i < n; i++ {
		arg, err := a.manager.GetArg(head, i)
		if err != nil {
			return none, out, err
		}
		used[arg] = true
	}
	kept := make([]ast.ASTHandle, 0, len(columns))
	for i, arg := range columns {
		if used[arg] {
			kept = append(kept, arg)
		} else {
			out.projection = append(out.projection, uint32(i))
		}
	}
	// ast_util.cpp flatten_and replaces the conjunction with its last child,
	// then processes the preceding children in order.
	for i := 1; i < tailCount; i++ {
		tailIndex := i
		if tailCount > 2 {
			if i == 1 {
				tailIndex = tailCount - 1
			} else {
				tailIndex = i - 1
			}
		}
		guard, neg, err := rule.Tail(tailIndex)
		if err != nil {
			return none, out, err
		}
		if neg {
			return none, out, ErrKarrDatalogFragment
		}
		condition, err := a.compileKarrGuardCondition(guard, positions, uint32(len(columns)))
		if err != nil {
			return none, out, err
		}
		if i == 1 {
			out.condition, out.positions = condition.condition, condition.positions
			out.row, out.constant, out.equal = condition.row, condition.constant, condition.equal
			out.secondRow, out.secondConstant = condition.secondRow, condition.secondConstant
		} else {
			out.more = append(out.more, condition)
		}
	}
	out.head, err = a.compileKarrHead(head, kept)
	if err != nil {
		return none, out, err
	}
	return hd, out, nil
}

func (a *karrDatalogActivation) applyGuardRule(r karrGuardRule, source *rel.KarrRelation) (*rel.KarrRelation, error) {
	result := source.Clone()
	for i, column := range r.inputConstants {
		if !result.Empty() {
			before := result.Clone()
			if err := result.FilterEqual(column, true); err != nil {
				return nil, err
			}
			if checker := result.FormulaChecker(); checker != nil {
				if i >= len(r.inputValues) {
					return nil, ErrKarrDatalogFragment
				}
				if err := checker.VerifyFilterEqual(before, column, r.inputValues[i], result); err != nil {
					return nil, err
				}
			}
		}
		var err error
		result, err = rel.KarrSourceProject(result, []uint32{column})
		if err != nil {
			return nil, err
		}
		if err = a.checkpoint(); err != nil {
			return nil, err
		}
	}
	if !result.Empty() && len(r.identical) > 0 {
		if err := result.FilterIdentical(r.identical); err != nil {
			return nil, err
		}
	}
	apply := func(condition karrGuardRule) error {
		before := result.Clone()
		if err := rel.KarrSourceFilterConstraint(result, condition.row, &condition.constant, condition.equal); err != nil {
			return err
		}
		if condition.secondRow != nil {
			if err := rel.KarrSourceFilterConstraint(result, condition.secondRow, &condition.secondConstant, false); err != nil {
				return err
			}
		}
		if checker := result.FormulaChecker(); checker != nil {
			translated, err := a.relationFilterCondition(condition.condition, condition.positions)
			if err != nil {
				return err
			}
			defer a.manager.DelRef(&translated)
			translatedHandle, err := a.manager.RefGet(translated)
			if err != nil {
				return err
			}
			if err = checker.VerifyFilter(before, translatedHandle, result); err != nil {
				return err
			}
		}
		return nil
	}
	if err := apply(r); err != nil {
		return nil, err
	}
	for _, condition := range r.more {
		if err := apply(condition); err != nil {
			return nil, err
		}
	}
	if err := a.checkpoint(); err != nil {
		return nil, err
	}
	if len(r.projection) > 0 {
		var err error
		result, err = rel.KarrSourceProject(result, r.projection)
		if err != nil {
			return nil, err
		}
		if err = a.checkpoint(); err != nil {
			return nil, err
		}
	}
	result, err := r.head.apply(result)
	if err != nil {
		return nil, err
	}
	if err := a.checkpoint(); err != nil {
		return nil, err
	}
	return result, nil
}
