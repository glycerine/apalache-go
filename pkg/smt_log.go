package apalache

import "fmt"

type SMTLogRecord interface {
	smtLogRecord()
}

type DeclareCellRecord struct{ Cell ArenaCell }
type DeclareMembershipRecord struct{ Set, Element ArenaCell }
type AssertGroundExpressionRecord struct{ Expression Expr }

func (DeclareCellRecord) smtLogRecord()            {}
func (DeclareMembershipRecord) smtLogRecord()      {}
func (AssertGroundExpressionRecord) smtLogRecord() {}

type SMTLogReplayTarget interface {
	DeclareCell(ArenaCell) error
	DeclareMembership(ArenaCell, ArenaCell) error
	Assert(Expr) error
}

// SMTLog is a parent-linked differential log of solver declarations and
// assertions. Records are replayed root-first and retain insertion order.
type SMTLog struct {
	parent  *SMTLog
	records []SMTLogRecord
}

func NewSMTLog(parent *SMTLog, records []SMTLogRecord) *SMTLog {
	return &SMTLog{parent: parent, records: append([]SMTLogRecord(nil), records...)}
}

func (log *SMTLog) Parent() *SMTLog { return log.parent }
func (log *SMTLog) Records() []SMTLogRecord {
	if log == nil {
		return nil
	}
	return append([]SMTLogRecord(nil), log.records...)
}

func (log *SMTLog) Replay(target SMTLogReplayTarget) error {
	if log == nil {
		return nil
	}
	if target == nil {
		return fmt.Errorf("SMT log replay requires a target")
	}
	if err := log.parent.Replay(target); err != nil {
		return err
	}
	for index, record := range log.records {
		var err error
		switch typed := record.(type) {
		case DeclareCellRecord:
			err = target.DeclareCell(typed.Cell)
		case DeclareMembershipRecord:
			err = target.DeclareMembership(typed.Set, typed.Element)
		case AssertGroundExpressionRecord:
			if typed.Expression == nil {
				err = fmt.Errorf("asserted expression cannot be nil")
			} else {
				err = target.Assert(typed.Expression)
			}
		default:
			err = fmt.Errorf("unknown SMT log record %T", record)
		}
		if err != nil {
			return fmt.Errorf("replay SMT log record %d: %w", index, err)
		}
	}
	return nil
}

func (log *SMTLog) RecursiveLength() int {
	if log == nil {
		return 0
	}
	return len(log.records) + log.parent.RecursiveLength()
}

var _ SMTLogReplayTarget = (*G3GroundEncoder)(nil)
