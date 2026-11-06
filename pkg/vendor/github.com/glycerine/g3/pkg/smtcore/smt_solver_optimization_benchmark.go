package smtcore

// Source: src/opt/opt_solver.cpp:62-68,157-193,453-481.

import (
	"io"
	"os"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/stopwatch"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var optimizationDumpCount uint32

func (s *SMTSolver) updateOptimizationDumpParameter(p *uparams.ParamsRef) (resultErr error) {
	owner := &s.optimization.parameters
	owner.Running = true
	defer func() { owner.Running = false }()
	helper, err := owner.BeginHelper(p, "opt")
	if err != nil {
		return err
	}
	defer func() { resultErr = owner.FinishHelper(helper, resultErr) }()
	value, err := owner.FrameBool(helper, "dump_benchmarks", false)
	if err == nil {
		s.optimization.dumpBenchmarks = value
	}
	return err
}

// File badbit is ignored by C++ iostreams here, while printer operands run.
type optimizationBenchmarkWriter struct {
	out    io.Writer
	failed bool
}

func (w *optimizationBenchmarkWriter) Write(p []byte) (int, error) {
	if !w.failed {
		n, err := w.out.Write(p)
		w.failed = err != nil || n != len(p)
	}
	return len(p), nil
}

func (s *SMTSolver) writeOptimizationBenchmark(out io.Writer, assumptions []ast.ASTHandle) (resultErr error) {
	printer, err := ast.NewLegacySMTPrinter(s.manager)
	if err != nil {
		return err
	}
	name, err := s.manager.Symbols.Intern("opt_solver")
	if err != nil {
		return err
	}
	owner := &s.optimization.parameters
	owner.Running = true
	defer func() { owner.Running = false }()
	empty := s.config.Params.EmptyValue()
	helper, err := owner.BeginHelper(&empty, "pp")
	if err != nil {
		return err
	}
	defer func() { resultErr = owner.FinishHelper(helper, resultErr) }()
	simplify, err := owner.FrameBool(helper, "simplify_implies", true)
	if err != nil {
		return err
	}
	if err = printer.SetSimplifyImplies(simplify); err != nil {
		return err
	}
	formulas := append([]ast.ASTHandle(nil), assumptions...)
	for _, assertion := range s.kernel.assertions {
		formulas = append(formulas, assertion.Formula)
	}
	truth, err := s.manager.MkTrue()
	if err != nil {
		return err
	}
	return printer.DisplayBenchmark(out, truth, ast.LegacySMTBenchmarkConfig{BenchmarkName: name, Assumptions: formulas})
}

func (s *SMTSolver) beginOptimizationBenchmark(watch *stopwatch.Stopwatch, assumptions []ast.ASTHandle) error {
	if s.optimization == nil || !s.optimization.dumpBenchmarks {
		return nil
	}
	if err := watch.Start(); err != nil {
		return err
	}
	optimizationDumpCount++
	name := "opt_solver" + strconv.FormatUint(uint64(optimizationDumpCount), 10) + ".smt2"
	file, openErr := os.Create(name)
	var out io.Writer = io.Discard
	if openErr == nil {
		out = file
	}
	err := s.writeOptimizationBenchmark(&optimizationBenchmarkWriter{out: out}, assumptions)
	if file != nil {
		_ = file.Close()
	}
	if err != nil {
		return err
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	_ = stream.AppendString("(created benchmark: " + name + "...")
	_ = stream.Flush()
	return nil
}

func (s *SMTSolver) finishOptimizationBenchmark(watch *stopwatch.Stopwatch, status lbool.LBool) error {
	if s.optimization == nil || !s.optimization.dumpBenchmarks {
		return nil
	}
	if err := watch.Stop(); err != nil {
		return err
	}
	enabled, err := diagnostic.ProcessVerboseEnabled(1)
	if err != nil || !enabled {
		return err
	}
	stream, err := diagnostic.BorrowProcessVerboseStream()
	if err != nil {
		return err
	}
	_ = stream.AppendString(".. " + status.String() + " ")
	if err = stream.SetFloatMode(utext.NumericFixedFloat); err != nil {
		return err
	}
	seconds, err := watch.GetSeconds()
	if err != nil {
		return err
	}
	_ = stream.AppendFloat64(seconds)
	_ = stream.AppendString(")\n")
	return nil
}
