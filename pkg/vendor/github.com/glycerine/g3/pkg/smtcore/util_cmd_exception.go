package smtcore

// Source: src/util/cmd_context_types.h. Shared concrete utility exceptions.

import (
	ucmdexception "github.com/glycerine/g3/pkg/util/cmdexception"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type CmdException = ucmdexception.CmdException
type StopParserException = ucmdexception.StopParserException

func NewCmdException(message string) CmdException { return ucmdexception.NewCmdException(message) }
func NewCmdExceptionCString(message string) CmdException {
	return ucmdexception.NewCmdExceptionCString(message)
}
func NewCmdExceptionAt(message string, line, pos int32) CmdException {
	return ucmdexception.NewCmdExceptionAt(message, line, pos)
}
func NewCmdExceptionSymbol(m *usymbol.Manager, message string, s usymbol.Symbol) (CmdException, error) {
	return ucmdexception.NewCmdExceptionSymbol(m, message, s)
}
func NewCmdExceptionSymbolAt(m *usymbol.Manager, message string, s usymbol.Symbol, line, pos int32) (CmdException, error) {
	return ucmdexception.NewCmdExceptionSymbolAt(m, message, s, line, pos)
}
