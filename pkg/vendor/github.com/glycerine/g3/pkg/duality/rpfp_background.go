package duality

import "github.com/glycerine/g3/pkg/ast"

// BackgroundVocabulary is the concrete constant set shared by graphs using
// one source LogicSolver (duality.h:319-331).
type BackgroundVocabulary struct {
	context   *Context
	constants map[ast.ASTHandle]ast.ASTRef
	closed    bool
}

func NewBackgroundVocabulary(context *Context) (*BackgroundVocabulary, error) {
	if err := context.open(); err != nil {
		return nil, err
	}
	return &BackgroundVocabulary{context: context, constants: make(map[ast.ASTHandle]ast.ASTRef)}, nil
}
func NewRPFPWithVocabulary(v *BackgroundVocabulary) (*RPFP, error) {
	if v == nil || v.closed {
		return nil, ErrWrapperOwner
	}
	if err := v.context.open(); err != nil {
		return nil, err
	}
	return &RPFP{context: v.context, vocabulary: v}, nil
}
func (g *RPFP) DeclareConstant(decl *FuncDecl) error {
	if err := g.open(); err != nil {
		return err
	}
	if g.vocabulary.closed {
		return ErrWrapperOwner
	}
	if err := decl.open(); err != nil {
		return err
	}
	if decl.value.object.context != g.context {
		return ErrContextMismatch
	}
	h, err := decl.Raw()
	if err != nil {
		return err
	}
	if _, found := g.vocabulary.constants[h]; found {
		return nil
	}
	ref, err := g.context.manager.CopyRef(decl.value.ref)
	if err != nil {
		return err
	}
	g.vocabulary.constants[h] = ref
	return nil
}
func (v *BackgroundVocabulary) Close() error {
	if v == nil || v.closed {
		return nil
	}
	for key, ref := range v.constants {
		err := v.context.manager.DelRef(&ref)
		if ref == (ast.ASTRef{}) {
			delete(v.constants, key)
		} else {
			v.constants[key] = ref
		}
		if err != nil {
			return err
		}
	}
	v.closed = true
	return nil
}
