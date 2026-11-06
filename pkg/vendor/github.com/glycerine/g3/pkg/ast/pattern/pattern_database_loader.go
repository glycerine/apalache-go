package pattern

// Source: src/ast/pattern/database.h; src/ast/pattern/expr_pattern_match.cpp:383-401.

import (
	_ "embed"
	"fmt"
	"strconv"
	"unicode"

	"github.com/glycerine/g3/pkg/ast"
)

//go:embed pattern_database.smt2
var embeddedPatternDatabase string

type databaseSExpr struct {
	atom string
	list []databaseSExpr
}

type patternDatabaseLoader struct {
	manager *ast.ASTManager
	matcher *exprPatternMatcher
	sorts   map[string]ast.ASTHandle
	decls   map[string]ast.ASTHandle
	owned   []ast.ASTHandle
	arith   ast.FamilyID
}

func parseDatabaseSExprs(text string) ([]databaseSExpr, error) {
	tokens := make([]string, 0, 2048)
	for i := 0; i < len(text); {
		if unicode.IsSpace(rune(text[i])) {
			i++
			continue
		}
		if text[i] == ';' {
			for i < len(text) && text[i] != '\n' {
				i++
			}
			continue
		}
		if text[i] == '(' || text[i] == ')' {
			tokens = append(tokens, text[i:i+1])
			i++
			continue
		}
		start := i
		for i < len(text) && !unicode.IsSpace(rune(text[i])) && text[i] != '(' && text[i] != ')' {
			i++
		}
		tokens = append(tokens, text[start:i])
	}
	var parse func(*int) (databaseSExpr, error)
	parse = func(at *int) (databaseSExpr, error) {
		if *at >= len(tokens) {
			return databaseSExpr{}, fmt.Errorf("pattern database: unexpected end of input")
		}
		token := tokens[*at]
		*at++
		if token != "(" {
			if token == ")" {
				return databaseSExpr{}, fmt.Errorf("pattern database: unexpected )")
			}
			return databaseSExpr{atom: token}, nil
		}
		n := databaseSExpr{}
		for {
			if *at >= len(tokens) {
				return databaseSExpr{}, fmt.Errorf("pattern database: unterminated list")
			}
			if tokens[*at] == ")" {
				*at++
				return n, nil
			}
			child, err := parse(at)
			if err != nil {
				return databaseSExpr{}, err
			}
			n.list = append(n.list, child)
		}
	}
	result := make([]databaseSExpr, 0, 64)
	for at := 0; at < len(tokens); {
		n, err := parse(&at)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, nil
}

func (l *patternDatabaseLoader) own(h ast.ASTHandle) error {
	if err := l.manager.IncRef(h); err != nil {
		return err
	}
	l.owned = append(l.owned, h)
	return nil
}

func (l *patternDatabaseLoader) close() (err error) {
	for len(l.owned) != 0 {
		i := len(l.owned) - 1
		h := l.owned[i]
		l.owned = l.owned[:i]
		if e := l.manager.DecRef(h); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (l *patternDatabaseLoader) declaration(n databaseSExpr) error {
	if len(n.list) != 4 || n.list[0].atom != "declare-fun" || n.list[1].atom == "" || n.list[2].atom != "" {
		return fmt.Errorf("pattern database: malformed declaration")
	}
	domain := make([]ast.ASTHandle, len(n.list[2].list))
	for i, sort := range n.list[2].list {
		var ok bool
		domain[i], ok = l.sorts[sort.atom]
		if !ok {
			return fmt.Errorf("pattern database: unknown domain sort %q", sort.atom)
		}
	}
	rangeSort, ok := l.sorts[n.list[3].atom]
	if !ok {
		return fmt.Errorf("pattern database: unknown range sort %q", n.list[3].atom)
	}
	name, err := l.manager.Symbols.Intern(n.list[1].atom)
	if err != nil {
		return err
	}
	decl, err := l.manager.MkFuncDecl(name, domain, rangeSort, ast.ASTFuncDeclInfo{})
	if err != nil {
		return err
	}
	if err = l.own(decl); err != nil {
		return err
	}
	l.decls[n.list[1].atom] = decl
	return nil
}

func (l *patternDatabaseLoader) numeral(text string) (result ast.ASTHandle, err error) {
	service := l.manager.NumeralService()
	if service == nil {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	value, err := service.NewValue()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	defer func() {
		if e := service.Del(&value); err == nil {
			err = e
		}
	}()
	if err = service.SetString(&value, text); err != nil {
		return ast.ASTHandle{}, err
	}
	return l.manager.ArithmeticNumeral(&value, true)
}

func (l *patternDatabaseLoader) expression(n databaseSExpr, bound map[string]ast.ASTHandle) (ast.ASTHandle, error) {
	if n.atom != "" {
		if variable, ok := bound[n.atom]; ok {
			return variable, nil
		}
		if decl, ok := l.decls[n.atom]; ok {
			return l.manager.MkApp(decl, nil)
		}
		if _, err := strconv.ParseInt(n.atom, 10, 32); err == nil {
			return l.numeral(n.atom)
		}
		return ast.ASTHandle{}, fmt.Errorf("pattern database: unknown atom %q", n.atom)
	}
	if len(n.list) == 0 || n.list[0].atom == "" {
		return ast.ASTHandle{}, fmt.Errorf("pattern database: malformed expression")
	}
	head := n.list[0].atom
	args := make([]ast.ASTHandle, len(n.list)-1)
	for i := 1; i < len(n.list); i++ {
		arg, err := l.expression(n.list[i], bound)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		args[i-1] = arg
	}
	if decl, ok := l.decls[head]; ok {
		return l.manager.MkApp(decl, args)
	}
	switch head {
	case "=":
		if len(args) != 2 {
			break
		}
		return l.manager.MkEq(args[0], args[1])
	case "iff":
		if len(args) != 2 {
			break
		}
		return l.manager.MkIff(args[0], args[1])
	case "not":
		if len(args) != 1 {
			break
		}
		return l.manager.MkNot(args[0])
	case "or":
		if len(args) < 2 {
			break
		}
		return l.manager.MkFamilyApp(l.manager.BasicFamilyID(), ast.ASTOpOr, nil, args)
	case "<=":
		if len(args) != 2 {
			break
		}
		return l.manager.MkFamilyApp(l.arith, 2, nil, args)
	case "+":
		if len(args) < 2 {
			break
		}
		return l.manager.MkFamilyApp(l.arith, 6, nil, args)
	case "-":
		if len(args) == 1 {
			return l.manager.MkFamilyApp(l.arith, 8, nil, args)
		}
		if len(args) == 2 {
			return l.manager.MkFamilyApp(l.arith, 7, nil, args)
		}
	case "*":
		if len(args) < 2 {
			break
		}
		return l.manager.MkFamilyApp(l.arith, 9, nil, args)
	}
	return ast.ASTHandle{}, fmt.Errorf("pattern database: unsupported application %q/%d", head, len(args))
}

func (l *patternDatabaseLoader) assertion(n databaseSExpr) error {
	if len(n.list) != 2 || n.list[0].atom != "assert" {
		return fmt.Errorf("pattern database: malformed assertion")
	}
	q := n.list[1]
	if len(q.list) != 3 || q.list[0].atom != "forall" || q.list[1].list == nil {
		return fmt.Errorf("pattern database: expected forall assertion")
	}
	bindings := q.list[1].list
	sorts := make([]ast.ASTHandle, len(bindings))
	bound := make(map[string]ast.ASTHandle, len(bindings))
	quantifierNames := make([]string, len(bindings))
	for i, binding := range bindings {
		if len(binding.list) != 2 || binding.list[0].atom == "" {
			return fmt.Errorf("pattern database: malformed binding")
		}
		name, sortName := binding.list[0].atom, binding.list[1].atom
		sort, ok := l.sorts[sortName]
		if !ok {
			return fmt.Errorf("pattern database: unknown binding sort %q", sortName)
		}
		variable, err := l.manager.MkVar(uint32(len(bindings)-i-1), sort)
		if err != nil {
			return err
		}
		sorts[i], bound[name], quantifierNames[i] = sort, variable, name
	}
	annotation := q.list[2]
	if len(annotation.list) < 4 || annotation.list[0].atom != "!" {
		return fmt.Errorf("pattern database: malformed quantifier annotation")
	}
	body, err := l.expression(annotation.list[1], bound)
	if err != nil {
		return err
	}
	patterns := make([]ast.ASTHandle, 0, 2)
	weight := int32(1)
	for i := 2; i < len(annotation.list); i += 2 {
		if i+1 >= len(annotation.list) {
			return fmt.Errorf("pattern database: annotation without value")
		}
		switch annotation.list[i].atom {
		case ":weight":
			v, e := strconv.ParseInt(annotation.list[i+1].atom, 10, 32)
			if e != nil {
				return fmt.Errorf("pattern database: invalid weight: %w", e)
			}
			weight = int32(v)
		case ":pattern":
			p := annotation.list[i+1]
			terms := []databaseSExpr{p}
			if len(p.list) != 0 && p.list[0].atom == "" {
				terms = p.list
			}
			expressions := make([]ast.ASTHandle, len(terms))
			for j := range terms {
				expressions[j], err = l.expression(terms[j], bound)
				if err != nil {
					return err
				}
			}
			pattern, e := l.manager.MkPattern(expressions)
			if e != nil {
				return e
			}
			patterns = append(patterns, pattern)
		default:
			return fmt.Errorf("pattern database: unsupported annotation %q", annotation.list[i].atom)
		}
	}
	if len(patterns) == 0 {
		return fmt.Errorf("pattern database: assertion has no pattern")
	}
	// ASTQuantifierSpec uses util/symbol.Symbol; build it without introducing a
	// parser-owned symbol representation.
	spec := ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Body: body, Weight: weight, Patterns: patterns}
	for _, text := range quantifierNames {
		symbol, e := l.manager.Symbols.Intern(text)
		if e != nil {
			return e
		}
		spec.Names = append(spec.Names, symbol)
	}
	quantifier, err := l.manager.MkQuantifier(spec)
	if err != nil {
		return err
	}
	return l.matcher.Compile(quantifier)
}

func (p *exprPatternMatcher) InitializeDatabase() (result error) {
	if err := p.open(); err != nil {
		return err
	}
	if len(p.instructions) != 0 {
		return nil
	}
	nodes, err := parseDatabaseSExprs(embeddedPatternDatabase)
	if err != nil {
		return err
	}
	l := patternDatabaseLoader{manager: p.manager, matcher: p, sorts: make(map[string]ast.ASTHandle), decls: make(map[string]ast.ASTHandle)}
	defer func() {
		if e := l.close(); result == nil {
			result = e
		}
	}()
	boolSort, err := p.manager.MkBoolSort()
	if err != nil {
		return err
	}
	l.arith, err = p.manager.RegisterArithmeticPlugin()
	if err != nil {
		return err
	}
	intSort, err := p.manager.MkFamilySort(l.arith, 1, nil)
	if err != nil {
		return err
	}
	l.sorts["Bool"], l.sorts["Int"] = boolSort, intSort
	for _, n := range nodes {
		if len(n.list) != 0 && n.list[0].atom == "declare-fun" {
			if err = l.declaration(n); err != nil {
				return err
			}
		}
	}
	for _, n := range nodes {
		if len(n.list) != 0 && n.list[0].atom == "assert" {
			if err = l.assertion(n); err != nil {
				return err
			}
		}
	}
	return nil
}
