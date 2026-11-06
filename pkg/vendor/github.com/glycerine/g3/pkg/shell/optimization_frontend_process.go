package shell

// Source: src/shell/opt_frontend.cpp timeout and SIGINT handling.

import (
	"errors"
	"os"
	"os/signal"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func runOptimizationFrontendProcess(
	context *cmd_context.Context,
	timer *TimeoutService,
	solve func(func() bool) (lbool.LBool, error),
	writeResult func(lbool.LBool) error,
	writeProgress func() error,
) (resultErr error) {
	if context == nil || solve == nil || writeResult == nil || writeProgress == nil {
		return ErrShellOptionService
	}
	if timer == nil {
		timer = BorrowProcessTimeoutService()
	}
	var timedOut atomic.Bool
	var interrupts atomic.Uint32
	if err := timer.RegisterOnTimeoutProc(func() {
		timedOut.Store(true)
		_ = context.SetCancel(true)
	}); err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, timer.RegisterOnTimeoutProc(nil)) }()

	interrupt := make(chan os.Signal, 2)
	stopInterrupt := make(chan struct{})
	interruptDone := make(chan struct{})
	signal.Notify(interrupt, os.Interrupt)
	go func() {
		defer close(interruptDone)
		for {
			select {
			case <-stopInterrupt:
				return
			case <-interrupt:
				if interrupts.Add(1) == 1 {
					_ = context.SetCancel(true)
				} else {
					return
				}
			}
		}
	}()
	defer func() {
		signal.Stop(interrupt)
		close(stopInterrupt)
		<-interruptDone
	}()

	status, err := solve(timedOut.Load)
	if timedOut.Load() {
		<-timer.Events()
		return writeProgress()
	}
	if err != nil {
		return err
	}
	if interrupts.Load() > 1 {
		signal.Stop(interrupt)
		return errors.Join(writeProgress(), raiseOptimizationFrontendInterrupt())
	}
	if err = writeResult(status); err != nil {
		return err
	}
	if interrupts.Load() > 1 {
		signal.Stop(interrupt)
		return errors.Join(writeProgress(), raiseOptimizationFrontendInterrupt())
	}
	return nil
}

func raiseOptimizationFrontendInterrupt() error {
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		return err
	}
	return process.Signal(os.Interrupt)
}
