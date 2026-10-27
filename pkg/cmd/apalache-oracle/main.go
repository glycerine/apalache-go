package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/glycerine/apalache-go/internal/oraclectl"
)

func main() {
	manifestPath := flag.String("manifest", "", "JSON differential-case manifest")
	scalaExecutable := flag.String("scala", "", "Scala apalache-mc executable")
	goExecutable := flag.String("go", "", "Go apalache executable")
	timeout := flag.Duration("timeout", 2*time.Minute, "timeout per differential case")
	flag.Parse()
	if *manifestPath == "" || *scalaExecutable == "" || *goExecutable == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	data, err := os.ReadFile(*manifestPath)
	if err != nil {
		fatal(err)
	}
	var manifest oraclectl.Manifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		fatal(err)
	}
	if len(manifest.Cases) == 0 {
		fatal(fmt.Errorf("oracle manifest has no cases"))
	}
	failed := false
	for _, test := range manifest.Cases {
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		result, runErr := oraclectl.RunDifferential(ctx, *scalaExecutable, *goExecutable, test)
		cancel()
		if runErr != nil {
			failed = true
			fmt.Fprintf(os.Stderr, "FAIL %s: %v\n", test.Name, runErr)
			continue
		}
		fmt.Printf("PASS %s: %s", test.Name, result.Go.Verdict)
		if result.Go.Depth >= 0 {
			fmt.Printf(" at depth %d", result.Go.Depth)
		}
		fmt.Println()
	}
	if failed {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
