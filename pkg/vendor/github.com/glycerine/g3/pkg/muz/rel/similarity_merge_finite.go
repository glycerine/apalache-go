package rel

// Source: dl_mk_similarity_compressor.cpp merge_class, finite table path.

import (
	"errors"
	"fmt"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
)

// similarityMergeFinite publishes auxiliary facts to the existing finite
// relation manager and returns the caller-owned representative rule. Domains
// not supported by that manager return an error before predicate creation.
func similarityMergeFinite(context *base.DatalogContext, relations *RelationManager, class similarityClass) (ast.ASTHandle, *base.Rule, error) {
	if context == nil {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	rm, e := context.RuleManager()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	return similarityMergeFiniteCore(context, rm, relations, class)
}

func similarityMergeFiniteCore(context *base.DatalogContext, rm *base.RuleManager, relations *RelationManager, class similarityClass) (result ast.ASTHandle, rule *base.Rule, err error) {
	return similarityMergeFiniteCoreWithTemplate(context, rm, relations, class, nil)
}

func similarityMergeFiniteCoreWithTemplate(context *base.DatalogContext, rm *base.RuleManager, relations *RelationManager, class similarityClass, template FiniteTable) (result ast.ASTHandle, rule *base.Rule, err error) {
	if rm == nil || relations == nil || !class.merge || len(class.rules) < 2 || len(class.constants) == 0 {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	if e := relations.open(); e != nil {
		return ast.ASTHandle{}, nil, e
	}
	m := relations.manager
	if context != nil {
		owner, e := context.Manager()
		if e != nil {
			return ast.ASTHandle{}, nil, e
		}
		if owner != m {
			return ast.ASTHandle{}, nil, ast.ErrASTDomain
		}
	}
	ruleManager, e := class.rules[0].ASTManager()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	if ruleManager != m {
		return ast.ASTHandle{}, nil, ast.ErrASTDomain
	}
	domains, rows, e := similarityOrphanRows(class.rules, class.constants)
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	// Validate and encode before registering a predicate so an unsupported
	// relation domain does not leave an unusable generated declaration behind.
	facts := make([]Fact, len(rows))
	for i, row := range rows {
		facts[i], e = relations.EncodeFact(domains, row)
		if e != nil {
			return ast.ASTHandle{}, nil, e
		}
	}
	head, e := class.rules[0].HeadDeclaration()
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	prefix, e := m.GetASTName(head)
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	suffix, e := m.Symbols.Intern(fmt.Sprintf("sc_%d", len(class.constants)))
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	var aux ast.ASTHandle
	if context != nil {
		aux, e = context.FreshHeadPredicate(prefix, suffix, domains)
	} else {
		var boolean ast.ASTHandle
		boolean, e = m.MkBoolSort()
		if e == nil {
			aux, e = m.MkFreshFuncDecl(prefix, suffix, domains, boolean)
		}
		if e == nil {
			var ref ast.ASTRef
			ref, e = m.NewRef(aux)
			if e == nil {
				defer func() {
					err = errors.Join(err, m.DelRef(&ref))
					if err != nil && rule != nil {
						err = errors.Join(err, rule.Close())
						rule = nil
					}
				}()
				e = rm.RegisterPredicate(aux)
			}
		}
	}
	if e != nil {
		return ast.ASTHandle{}, nil, e
	}
	var table FiniteTable
	if template == nil {
		table, e = relations.GetFiniteRelation(aux)
	} else {
		// mk_fresh_head_predicate inherits the explicitly requested source
		// relation kind before add_fact first constructs the auxiliary owner.
		var signature Signature
		signature, e = tableRelationSignature(relations, domains)
		if e == nil {
			table, e = newFiniteTableLike(template, signature)
		}
		if e == nil {
			if e = relations.StoreFiniteRelation(aux, table); e != nil {
				e = errors.Join(e, table.Close())
			}
		}
	}
	if e != nil {
		return aux, nil, e
	}
	for _, fact := range facts {
		if _, e = table.Add(fact); e != nil {
			return aux, nil, e
		}
	}
	if e = relations.MarkSaturated(aux); e != nil {
		return aux, nil, e
	}
	rule, e = similarityRewriteRule(rm, class.rules[0], class.constants, aux)
	return aux, rule, e
}
