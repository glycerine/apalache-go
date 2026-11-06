package shell

// Source: src/shell/main.cpp:169-185.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/version"
)

func WriteVersion(out io.Writer) error {
	if out == nil {
		return ErrShellOptionService
	}
	text := "Z3 version " + strconv.FormatUint(uint64(version.Major), 10) + "." + strconv.FormatUint(uint64(version.Minor), 10) + "." + strconv.FormatUint(uint64(version.Build), 10) + " - " + shellBitWidth() + " bit\n"
	return writeShellText(out, text)
}
