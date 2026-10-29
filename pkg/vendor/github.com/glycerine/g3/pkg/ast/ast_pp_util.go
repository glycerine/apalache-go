package ast

// Source: src/ast/ast_pp_util.cpp; src/ast/ast_pp_util.h.

import "io"

type ASTPPUtil struct {
	manager      *ASTManager
	collector    *astDeclCollector
	removed      map[ASTHandle]bool
	removedSorts map[ASTHandle]bool
}

func NewASTPPUtil(manager *ASTManager) (*ASTPPUtil, error) {
	if manager == nil {
		return nil, ErrASTHandle
	}
	if _, err := manager.ProofsEnabled(); err != nil {
		return nil, err
	}
	return &ASTPPUtil{
		manager: manager, collector: newASTDeclCollector(manager, false), removed: make(map[ASTHandle]bool), removedSorts: make(map[ASTHandle]bool),
	}, nil
}

// RemoveSort suppresses a previously emitted sort while retaining it as a
// dependency for ordering declarations collected later.
func (p *ASTPPUtil) RemoveSort(sort ASTHandle) error {
	if p == nil || p.manager == nil || p.removedSorts == nil {
		return ErrASTHandle
	}
	if kind, err := p.manager.GetKind(sort); err != nil {
		return err
	} else if kind != ASTSort {
		return ErrASTDomain
	}
	p.removedSorts[sort] = true
	return nil
}

// CollectedDeclarations returns borrowed declarations reached so far.
func (p *ASTPPUtil) CollectedDeclarations() ([]ASTHandle, error) {
	if p == nil || p.collector == nil {
		return nil, ErrASTHandle
	}
	return append([]ASTHandle(nil), p.collector.decls...), nil
}

// CollectedSorts returns borrowed user and datatype sorts reached so far.
func (p *ASTPPUtil) CollectedSorts() ([]ASTHandle, error) {
	if p == nil || p.collector == nil {
		return nil, ErrASTHandle
	}
	return append([]ASTHandle(nil), p.collector.sorts...), nil
}

func (p *ASTPPUtil) Collect(expr ASTHandle) error {
	if p == nil || p.manager == nil || p.collector == nil {
		return ErrASTHandle
	}
	return p.collector.visit(expr)
}

func (p *ASTPPUtil) CollectAll(exprs []ASTHandle) error {
	for _, expr := range exprs {
		if err := p.Collect(expr); err != nil {
			return err
		}
	}
	return nil
}

func (p *ASTPPUtil) RemoveDeclaration(decl ASTHandle) error {
	if p == nil || p.manager == nil || p.removed == nil {
		return ErrASTHandle
	}
	if kind, err := p.manager.GetKind(decl); err != nil {
		return err
	} else if kind != ASTFuncDecl {
		return ErrASTDomain
	}
	p.removed[decl] = true
	return nil
}

func (p *ASTPPUtil) DisplayDeclarations(out io.Writer) error {
	if p == nil || p.manager == nil || p.collector == nil || out == nil {
		return ErrASTHandle
	}
	if err := p.collector.orderDeps(); err != nil {
		return err
	}
	legacy, err := NewLegacySMTPrinter(p.manager)
	if err != nil {
		return err
	}
	modern, err := NewSimpleASTPrinterContext(p.manager, out, nil)
	if err != nil {
		return err
	}
	for _, sort := range p.collector.sorts {
		if p.removedSorts[sort] {
			continue
		}
		if err = legacy.DisplayAST(out, sort, 0, nil); err != nil {
			return err
		}
	}
	for _, decl := range p.collector.decls {
		if p.removed[decl] {
			continue
		}
		family, e := p.manager.GetFamilyID(decl)
		if e != nil {
			return e
		}
		if family != NullFamilyID {
			continue
		}
		format, e := modern.PrettyFuncDecl(decl)
		if e != nil {
			return e
		}
		if e = modern.displayFormat(out, format, 0); e != nil {
			return e
		}
		if e = legacySMTWriteText(out, "\n"); e != nil {
			return e
		}
	}
	return nil
}

func (p *ASTPPUtil) DisplayAssertions(out io.Writer, formulas []ASTHandle, neat bool) error {
	if p == nil || p.manager == nil || out == nil {
		return ErrASTHandle
	}
	modern, err := NewSimpleASTPrinterContext(p.manager, out, nil)
	if err != nil {
		return err
	}
	legacy, err := NewLegacySMTPrinter(p.manager)
	if err != nil {
		return err
	}
	for _, formula := range formulas {
		if err = legacySMTWriteText(out, "(assert "); err != nil {
			return err
		}
		if neat {
			format, e := modern.PrettyExpr(formula)
			if e != nil {
				return e
			}
			if e = modern.displayFormat(out, format, 0); e != nil {
				return e
			}
		} else if err = legacy.DisplayExpr(out, formula, nil); err != nil {
			return err
		}
		if err = legacySMTWriteText(out, ")\n"); err != nil {
			return err
		}
	}
	return nil
}
