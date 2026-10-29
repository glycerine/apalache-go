package smtcore

// Source: src/smt/qi_queue.cpp:202-288, excluding proof construction and the
// final context::internalize_instance call.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type SMTQIPreparedInstance struct {
	manager          *ast.ASTManager
	entry            SMTQIQueueEntry
	quantifier       ast.ASTHandle
	instance         ast.ASTRef
	alreadySatisfied bool
	closed           bool
}

func (p *SMTQIPreparedInstance) AlreadySatisfied() bool {
	return p != nil && p.alreadySatisfied
}

func (p *SMTQIPreparedInstance) Instance() (ast.ASTHandle, error) {
	if p == nil || p.manager == nil || p.closed || p.alreadySatisfied {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return p.manager.RefGet(p.instance)
}

func (p *SMTQIPreparedInstance) Close() error {
	if p == nil || p.manager == nil {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	var err error
	if !p.alreadySatisfied {
		err = p.manager.DelRef(&p.instance)
	}
	p.closed = err == nil
	return err
}

type SMTQIInstance struct {
	manager     *ast.ASTManager
	fingerprint *SMTFingerprint
	quantifier  ast.ASTHandle
	lemma       ast.ASTRef
	proof       ast.ASTRef
	generation  uint32
	reducedTrue bool
	hasProof    bool
	closed      bool
}

func (i *SMTQIInstance) Fingerprint() *SMTFingerprint {
	if i == nil {
		return nil
	}
	return i.fingerprint
}

func (i *SMTQIInstance) Quantifier() ast.ASTHandle {
	if i == nil {
		return ast.ASTHandle{}
	}
	return i.quantifier
}

func (i *SMTQIInstance) ReducedTrue() bool { return i != nil && i.reducedTrue }
func (i *SMTQIInstance) Generation() uint32 {
	if i == nil {
		return 0
	}
	return i.generation
}

func (i *SMTQIInstance) Lemma() (ast.ASTHandle, error) {
	if i == nil || i.manager == nil || i.closed || i.reducedTrue {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	return i.manager.RefGet(i.lemma)
}

func (i *SMTQIInstance) Proof() (ast.ASTHandle, error) {
	if i == nil || i.manager == nil || i.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if !i.hasProof {
		return ast.ASTHandle{}, nil
	}
	return i.manager.RefGet(i.proof)
}

func (i *SMTQIInstance) Close() error {
	if i == nil || i.manager == nil {
		return ast.ErrASTHandle
	}
	if i.closed {
		return nil
	}
	var err error
	if i.hasProof {
		err = errors.Join(err, i.manager.DelRef(&i.proof))
	}
	if !i.reducedTrue {
		err = errors.Join(err, i.manager.DelRef(&i.lemma))
	}
	i.closed = err == nil
	return err
}

type SMTQIInstanceBuilder struct {
	manager      *ast.ASTManager
	checker      *SMTChecker
	substitution *SMTCachedVarSubstitution
	costs        *SMTQICostModel
}

func NewSMTQIInstanceBuilder(manager *ast.ASTManager, checker *SMTChecker, substitution *SMTCachedVarSubstitution, costs *SMTQICostModel) (*SMTQIInstanceBuilder, error) {
	if manager == nil || checker == nil || substitution == nil || costs == nil {
		return nil, ast.ErrASTHandle
	}
	if checker.context == nil || checker.context.manager != manager || substitution.manager != manager || costs.manager != manager {
		return nil, ast.ErrASTDomain
	}
	return &SMTQIInstanceBuilder{manager: manager, checker: checker, substitution: substitution, costs: costs}, nil
}

func (b *SMTQIInstanceBuilder) Prepare(entry SMTQIQueueEntry) (*SMTQIPreparedInstance, error) {
	if b == nil || b.manager == nil || b.checker == nil || b.substitution == nil {
		return nil, ast.ErrASTHandle
	}
	if entry.Fingerprint == nil || entry.Fingerprint.data.IsNull() {
		return nil, ast.ErrASTDomain
	}
	quantifier := entry.Fingerprint.data
	kind, err := b.manager.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		if err != nil {
			return nil, err
		}
		return nil, ast.ErrASTDomain
	}
	body, err := b.manager.GetQuantifierBody(quantifier)
	if err != nil {
		return nil, err
	}
	bindings := entry.Fingerprint.args
	satisfied, err := b.checker.IsSatisfiable(body, bindings)
	if err != nil {
		return nil, err
	}
	prepared := &SMTQIPreparedInstance{manager: b.manager, entry: entry, quantifier: quantifier, alreadySatisfied: satisfied}
	if satisfied {
		return prepared, nil
	}
	prepared.instance, err = b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	if _, err = b.substitution.Instantiate(quantifier, bindings, prepared.instance); err != nil {
		return nil, errors.Join(err, prepared.Close())
	}
	return prepared, nil
}

// Materialize invokes the context theory rewriter and completes the source
// instance lemma. A nil result means the current context already satisfies the
// substituted body.
func (b *SMTQIInstanceBuilder) Materialize(prepared *SMTQIPreparedInstance, rw *rewriter.TheoryRewriter, stat *SMTQuantifierStat, scope uint32) (result *SMTQIInstance, err error) {
	if b == nil || b.manager == nil || prepared == nil || prepared.manager != b.manager || prepared.closed || rw == nil || rw.Manager() != b.manager || stat == nil {
		return nil, ast.ErrASTDomain
	}
	if prepared.alreadySatisfied {
		return nil, nil
	}
	instance, err := prepared.Instance()
	if err != nil {
		return nil, err
	}
	simplified, err := b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, b.manager.DelRef(&simplified)) }()
	proofs, err := b.manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if !proofs {
		if err = rw.Rewrite(instance, simplified); err != nil {
			return nil, err
		}
		rewritten, err := b.manager.RefGet(simplified)
		if err != nil {
			return nil, err
		}
		return b.Finish(prepared, rewritten, stat, scope)
	}
	rewriteProof, err := b.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, b.manager.DelRef(&rewriteProof)) }()
	if err = rw.RewriteWithProof(instance, simplified, rewriteProof); err != nil {
		return nil, err
	}
	rewritten, err := b.manager.RefGet(simplified)
	if err != nil {
		return nil, err
	}
	proof, err := b.manager.RefGet(rewriteProof)
	if err != nil {
		return nil, err
	}
	return b.FinishWithProof(prepared, rewritten, proof, stat, scope)
}

// MaterializeAndInternalize completes qi_queue::instantiate through the
// concrete SMT internalizer. The returned object retains the source queue's
// lemma/proof ownership until its caller closes or scopes it.
func (b *SMTQIInstanceBuilder) MaterializeAndInternalize(prepared *SMTQIPreparedInstance, rw *rewriter.TheoryRewriter, stat *SMTQuantifierStat, scope uint32, internalizer *SMTInternalizerState) (*SMTQIInstance, error) {
	if internalizer == nil || internalizer.manager != b.manager {
		return nil, ast.ErrASTDomain
	}
	result, err := b.Materialize(prepared, rw, stat, scope)
	if err != nil {
		return result, fmt.Errorf("rewrite and finish instance: %w", err)
	}
	if result == nil {
		return nil, nil
	}
	if err = internalizer.InternalizeInstance(result); err != nil {
		return nil, errors.Join(fmt.Errorf("internalize instance lemma: %w", err), result.Close())
	}
	return result, nil
}

func (b *SMTQIInstanceBuilder) Finish(prepared *SMTQIPreparedInstance, simplified ast.ASTHandle, stat *SMTQuantifierStat, scope uint32) (*SMTQIInstance, error) {
	if b == nil || b.manager == nil {
		return nil, ast.ErrASTHandle
	}
	proofs, err := b.manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if proofs {
		return nil, ast.ErrASTDomain
	}
	return b.finishLemma(prepared, simplified, stat, scope)
}

func (b *SMTQIInstanceBuilder) finishLemma(prepared *SMTQIPreparedInstance, simplified ast.ASTHandle, stat *SMTQuantifierStat, scope uint32) (*SMTQIInstance, error) {
	if b == nil || b.manager == nil || prepared == nil || prepared.manager != b.manager || prepared.closed || prepared.alreadySatisfied || stat == nil {
		return nil, ast.ErrASTDomain
	}
	sort, err := b.manager.GetSort(simplified)
	if err != nil {
		return nil, err
	}
	boolean, err := b.manager.MkBoolSort()
	if err != nil {
		return nil, err
	}
	if sort != boolean {
		return nil, ast.ErrASTDomain
	}
	result := &SMTQIInstance{manager: b.manager, fingerprint: prepared.entry.Fingerprint, quantifier: prepared.quantifier}
	if b.manager.IsTrue(simplified) {
		result.reducedTrue = true
		return result, nil
	}
	stat.IncrementInstances()
	negatedQuantifier, err := b.manager.MkNot(prepared.quantifier)
	if err != nil {
		return nil, err
	}
	lemma := negatedQuantifier
	if !b.manager.IsFalse(simplified) {
		kind, err := b.manager.GetKind(simplified)
		if err != nil {
			return nil, err
		}
		arguments := []ast.ASTHandle{negatedQuantifier}
		if kind == ast.ASTApp {
			family, err := b.manager.GetFamilyID(simplified)
			if err != nil {
				return nil, err
			}
			op, err := b.manager.GetDeclKind(simplified)
			if err != nil {
				return nil, err
			}
			if family == b.manager.BasicFamilyID() && op == ast.ASTOpOr {
				count, err := b.manager.GetNumArgs(simplified)
				if err != nil {
					return nil, err
				}
				for index := uint32(0); index < count; index++ {
					argument, err := b.manager.GetArg(simplified, index)
					if err != nil {
						return nil, err
					}
					arguments = append(arguments, argument)
				}
			} else {
				arguments = append(arguments, simplified)
			}
		} else {
			arguments = append(arguments, simplified)
		}
		lemma, err = b.manager.MkOrArgs(arguments)
		if err != nil {
			return nil, err
		}
	}
	result.generation, err = b.costs.NewGeneration(prepared.quantifier, stat, prepared.entry.Generation, scope, prepared.entry.Cost)
	if err != nil {
		return nil, err
	}
	result.lemma, err = b.manager.NewRef(lemma)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// FinishWithProof constructs the source quantifier-instantiation proof after
// the enclosing context has rewritten the prepared instance. rewriteProof is
// the context rewriter's proof that Instance() equals simplified.
func (b *SMTQIInstanceBuilder) FinishWithProof(prepared *SMTQIPreparedInstance, simplified, rewriteProof ast.ASTHandle, stat *SMTQuantifierStat, scope uint32) (*SMTQIInstance, error) {
	if b == nil || b.manager == nil {
		return nil, ast.ErrASTHandle
	}
	proofs, err := b.manager.ProofsEnabled()
	if err != nil {
		return nil, err
	}
	if !proofs {
		return b.finishLemma(prepared, simplified, stat, scope)
	}
	result, err := b.finishLemma(prepared, simplified, stat, scope)
	if err != nil || result.reducedTrue {
		return result, err
	}
	instance, err := prepared.Instance()
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	lemma, err := result.Lemma()
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	negatedQuantifier, err := b.manager.MkNot(prepared.quantifier)
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	bareLemma, err := b.manager.MkOr(negatedQuantifier, instance)
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	bindings := make([]ast.ASTHandle, len(prepared.entry.Fingerprint.args))
	for index, binding := range prepared.entry.Fingerprint.args {
		if binding == nil || binding.owner.IsNull() {
			_ = result.Close()
			return nil, ast.ErrASTDomain
		}
		bindings[index] = binding.owner
	}
	proof, err := b.manager.MkProofQuantInstantiation(bareLemma, bindings)
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	if bareLemma != lemma {
		if instance == simplified {
			rewrite, err := b.manager.MkProofRewrite(bareLemma, lemma)
			if err != nil {
				_ = result.Close()
				return nil, err
			}
			proof, err = b.manager.MkProofModusPonens(proof, rewrite)
			if err != nil {
				_ = result.Close()
				return nil, err
			}
		} else {
			if rewriteProof.IsNull() {
				_ = result.Close()
				return nil, ast.ErrASTDomain
			}
			bareSimplifiedLemma, err := b.manager.MkOr(negatedQuantifier, simplified)
			if err != nil {
				_ = result.Close()
				return nil, err
			}
			congruence, err := b.manager.MkProofCongruence(bareLemma, bareSimplifiedLemma, []ast.ASTHandle{rewriteProof})
			if err != nil {
				_ = result.Close()
				return nil, err
			}
			rewrite, err := b.manager.MkProofRewrite(bareSimplifiedLemma, lemma)
			if err != nil {
				_ = result.Close()
				return nil, err
			}
			transitivity, err := b.manager.MkProofTransitivity(congruence, rewrite)
			if err != nil {
				_ = result.Close()
				return nil, err
			}
			proof, err = b.manager.MkProofModusPonens(proof, transitivity)
			if err != nil {
				_ = result.Close()
				return nil, err
			}
		}
	}
	result.proof, err = b.manager.NewRef(proof)
	if err != nil {
		_ = result.Close()
		return nil, err
	}
	result.hasProof = true
	return result, nil
}
