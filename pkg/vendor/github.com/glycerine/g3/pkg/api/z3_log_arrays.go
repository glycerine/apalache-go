package api

import (
	"errors"
	"strconv"
)

var ErrZ3LogSymbol = errors.New("invalid z3 log symbol")

func (log *Z3LogWriter) PointerArray(size uint32) error {
	return log.writeRecord("p " + strconv.FormatUint(uint64(size), 10) + "\n")
}

func (log *Z3LogWriter) UintArray(size uint32) error {
	return log.writeRecord("u " + strconv.FormatUint(uint64(size), 10) + "\n")
}

func (log *Z3LogWriter) SymbolArray(size uint32) error {
	return log.writeRecord("s " + strconv.FormatUint(uint64(size), 10) + "\n")
}
