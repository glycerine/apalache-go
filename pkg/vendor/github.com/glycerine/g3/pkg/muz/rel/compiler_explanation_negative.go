package rel

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// ValidateExplanationNegativeRelations checks that an original rule-defined
// negative predicate has an actual finite producer or output bridge. The
// source explanation transform leaves negative literals unchanged. Validate
// before pruning transforms such as magic sets can remove their explained
// producers and obscure the missing bridge.
func ValidateExplanationNegativeRelations(set *base.RuleSet) error {
	if set == nil {
		return ErrCompilerUnsupported
	}
	count, err := set.Len()
	if err != nil {
		return err
	}
	heads := make(map[ast.ASTHandle]bool)
	var negatives []ast.ASTHandle
	for index := 0; index < count; index++ {
		rule, err := set.Rule(index)
		if err != nil {
			return err
		}
		head, err := rule.HeadDeclaration()
		if err == nil {
			heads[head] = true
		}
		positive, pErr := rule.PositiveTailSize()
		uninterpreted, uErr := rule.UninterpretedTailSize()
		err = errors.Join(err, pErr, uErr)
		for tail := positive; err == nil && tail < uninterpreted; tail++ {
			predicate, tailErr := rule.TailDeclaration(tail)
			err = tailErr
			if err == nil {
				negatives = append(negatives, predicate)
			}
		}
		if err = errors.Join(err, rule.Close()); err != nil {
			return err
		}
	}
	for _, negative := range negatives {
		derived, err := set.DerivedPredicate(negative)
		if err != nil {
			return err
		}
		if derived != negative && heads[derived] && !heads[negative] {
			return ErrCompilerUnsupported
		}
	}
	return nil
}
