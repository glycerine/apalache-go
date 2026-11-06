package clp

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"sort"
)

func closeRules(rs []*base.Rule) (err error) {
	for _, r := range rs {
		err = errors.Join(err, r.Close())
	}
	return err
}
func sortedRules(set *base.RuleSet, decl ast.ASTHandle) ([]*base.Rule, error) {
	rs, err := set.PredicateRules(decl)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rs, func(i, j int) bool {
		a, _ := rs[i].UninterpretedTailSize()
		b, _ := rs[j].UninterpretedTailSize()
		return a < b
	})
	return rs, nil
}
