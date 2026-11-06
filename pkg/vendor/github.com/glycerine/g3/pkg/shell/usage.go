package shell

// Source: src/shell/main.cpp:56-104; src/shell/options.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/version"
)

func writeShellText(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err != nil {
		return err
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}

func shellBitWidth() string { return strconv.Itoa(strconv.IntSize) }

// WriteUsage emits the source command-line help in source section order.
func WriteUsage(out io.Writer) error {
	if out == nil {
		return ErrShellOptionService
	}
	header := "Z3 [version " + strconv.FormatUint(uint64(version.Major), 10) + "." + strconv.FormatUint(uint64(version.Minor), 10) + "." + strconv.FormatUint(uint64(version.Build), 10) + " - " + shellBitWidth() + " bit]. (C) Copyright 2006-2016 Microsoft Corp.\n" +
		"Usage: z3 [options] [-file:]file\n"
	if err := writeShellText(out, header); err != nil {
		return err
	}
	for _, section := range []func(io.Writer) error{writeUsageInput, writeUsageMisc, writeUsageResources, writeUsageOutput, writeUsageParameters} {
		if err := section(out); err != nil {
			return err
		}
	}
	return nil
}
