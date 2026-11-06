package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp:643-694.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// Lexical owners remain reachable when a destructor stops during a failed
// construction, evaluation or conversion. Close retries the actual owner.
type modelConversion struct {
	md            *model.ModelCore
	mdCleanup     bool
	evaluator     *model.ModelEvaluator
	value         ast.ASTRef
	valueReady    bool
	formatManager *ast.ASTManager
	formats       []ast.ASTHandle
	stream        *utext.NumericTextStream
}

func (s *Solver) drainModelWork() error {
	for len(s.modelWork) > 0 {
		i := len(s.modelWork) - 1
		w := s.modelWork[i]
		if w.valueReady {
			if err := s.manager.DelRef(&w.value); err != nil {
				return err
			}
			w.valueReady = false
		}
		if w.evaluator != nil {
			if err := w.evaluator.Close(); err != nil {
				return err
			}
			w.evaluator = nil
		}
		if w.md != nil {
			if w.mdCleanup {
				if err := w.md.Close(); err != nil {
					return err
				}
			} else {
				consumed, err := w.md.DecRefConsumed()
				if consumed && err != nil {
					w.mdCleanup = true
				}
				if err != nil {
					return err
				}
			}
			w.md = nil
		}
		if w.stream != nil {
			if err := w.stream.Close(); err != nil {
				return err
			}
			w.stream = nil
		}
		if w.formatManager != nil {
			if err := releaseRoots(w.formatManager, &w.formats, 0); err != nil {
				return err
			}
		}
		s.modelWork = s.modelWork[:i]
	}
	return nil
}

// AcquireModel returns a counted model, independent of this solver's lifetime.
func (s *Solver) AcquireModel() (result *model.ModelCore, err error) {
	if err = s.open(); err != nil {
		return nil, err
	}
	if s.modelPending {
		return nil, ast.ErrASTPending
	}
	md, err := s.model.Get()
	if err != nil {
		return nil, err
	}
	if md == nil {
		if err = s.extractModel(); err != nil {
			return nil, err
		}
		md, err = s.model.Get()
		if err != nil {
			return nil, err
		}
	}
	if md != nil {
		if err = md.IncRef(); err != nil {
			return nil, err
		}
	}
	return md, nil
}

func (s *Solver) extractModel() (err error) {
	defer func() { s.retainFailure(err); err = errors.Join(err, s.drainModelWork(), s.drainConverters()) }()
	current, err := s.core.SourceModelIsCurrent()
	if err != nil {
		return err
	}
	if err = s.emit("extract_model", 644, func(out io.Writer) error {
		state := "absent"
		if current {
			state = "present"
		}
		_, e := io.WriteString(out, "retrieve model "+state+"\n")
		return e
	}); err != nil {
		return err
	}
	if !current {
		return s.model.Set(nil)
	}
	values, err := s.core.AppendModel(nil)
	if err != nil {
		return err
	}
	md, err := model.NewModel(s.manager)
	if err != nil {
		return err
	}
	w := &modelConversion{md: md.Core()}
	s.modelWork = append(s.modelWork, w)
	if err = w.md.IncRef(); err != nil {
		w.mdCleanup = true
		return err
	}
	entries, err := s.encoder.atoms.Entries()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		n, e := s.manager.GetNumArgs(entry.Expr)
		if e != nil {
			return e
		}
		if n > 0 {
			continue
		}
		v, e := values.Read(sat.BoolVar(entry.Variable))
		if e != nil {
			return e
		}
		if v == lbool.Undef {
			continue
		}
		d, e := s.manager.GetDecl(entry.Expr)
		if e != nil {
			return e
		}
		var value ast.ASTHandle
		if v == lbool.True {
			value, e = s.manager.MkTrue()
		} else {
			value, e = s.manager.MkFalse()
		}
		if e != nil {
			return e
		}
		if e = w.md.RegisterConstant(d, value); e != nil {
			return e
		}
	}
	if err = s.model.Set(w.md); err != nil {
		return err
	}
	s.modelPending = true
	bindings, err := s.encoder.BitVectorBindings()
	if err != nil {
		return err
	}
	if len(bindings) > 0 {
		pairs := make([]bv.BitBlasterBinding, 0, len(bindings))
		for d, bits := range bindings {
			pairs = append(pairs, bv.BitBlasterBinding{Variable: d, Bits: bits})
		}
		fresh, e := bv.NewBitBlasterModelConverter(s.manager, s.services.Context, s.services.Globals, pairs)
		if e != nil {
			return e
		}
		local := &converterOwner{value: fresh, cleanup: true}
		s.converters = append(s.converters, local)
		if e = fresh.IncRef(); e != nil {
			return e
		}
		local.cleanup = false
		var old tactic.ModelConverter
		if s.mc0 != nil {
			old = s.mc0.value
		}
		combined, e := tactic.ConcatModels(s.manager.Budget(), old, fresh)
		if e != nil {
			return e
		}
		owner := &converterOwner{value: combined, cleanup: combined != fresh}
		if combined != fresh {
			s.converters = append(s.converters, owner)
		}
		if e = combined.IncRef(); e != nil {
			return e
		}
		owner.cleanup = false
		if combined != fresh {
			s.converters = s.converters[:len(s.converters)-1]
		}
		if s.mc0 != nil {
			s.converters = append(s.converters, s.mc0)
		}
		s.mc0 = owner
	}
	if s.mc0 != nil {
		if err = s.mc0.value.ApplyModel(&s.model, 0); err != nil {
			return err
		}
	}
	if ast.ASTSourceDebug {
		current, e := s.model.Get()
		if e != nil {
			return e
		}
		w.evaluator, e = model.NewModelEvaluator(current, s.services)
		if e != nil {
			return e
		}
		if e = w.evaluator.SetModelCompletion(true); e != nil {
			return e
		}
		w.value, e = s.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		w.valueReady = true
		for _, f := range s.formulas {
			if e = w.evaluator.Evaluate(f, w.value); e != nil {
				return e
			}
			v, e := s.manager.RefGet(w.value)
			if e != nil {
				return e
			}
			if !s.manager.IsTrue(v) {
				if e = s.emit("extract_model", 682, func(out io.Writer) error {
					if _, e := io.WriteString(out, "Evaluation failed: "); e != nil {
						return e
					}
					if e := s.writeExpression(out, f); e != nil {
						return e
					}
					if _, e := io.WriteString(out, " to "); e != nil {
						return e
					}
					if e := s.writeExpression(out, v); e != nil {
						return e
					}
					if _, e := io.WriteString(out, "\n"); e != nil {
						return e
					}
					return current.WriteSMT2(out, s.services.Context, s.services.Globals, 0)
				}); e != nil {
					return e
				}
				return ast.ErrASTDomain
			}
		}
	}
	s.modelPending = false
	return nil
}

// expr_ref_vector printing uses mk_seq, including a leading line break and no
// enclosing parentheses; the pretty-printer uses its default parameter set.
func (s *Solver) formatExpressions(expressions []ast.ASTHandle) (result string, err error) {
	w := &modelConversion{}
	s.modelWork = append(s.modelWork, w)
	defer func() { s.retainFailure(err); err = errors.Join(err, s.drainModelWork()) }()
	w.formatManager, err = s.manager.GetFormatManager()
	if err != nil {
		return "", err
	}
	var docs []ast.ASTHandle
	for _, expression := range expressions {
		doc, e := s.manager.SMT2PPExprTree(expression, nil)
		if e != nil {
			return "", e
		}
		if e = w.formatManager.IncRef(doc); e != nil {
			return "", e
		}
		w.formats = append(w.formats, doc)
		line, e := s.manager.FormatLineBreak()
		if e != nil {
			return "", e
		}
		if e = w.formatManager.IncRef(line); e != nil {
			return "", e
		}
		w.formats = append(w.formats, line)
		docs = append(docs, line, doc)
	}
	root, err := s.manager.FormatCompose(docs)
	if err != nil {
		return "", err
	}
	if err = w.formatManager.IncRef(root); err != nil {
		return "", err
	}
	w.formats = append(w.formats, root)
	w.stream, err = utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: s.manager.Budget()})
	if err != nil {
		return "", err
	}
	empty := s.services.Context.EmptyValue()
	if err = s.manager.PrintFormat(w.stream, root, s.services.Context, &empty, s.services.Globals); err != nil {
		return "", err
	}
	return w.stream.String()
}
