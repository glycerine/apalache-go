//go:build oracle

package apalache

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/glycerine/apalache-go/internal/oraclectl"
)

func TestScalaDifferentialSafetyManifest(t *testing.T) {
	scalaExecutable := os.Getenv("APALACHE_SCALA")
	goExecutable := os.Getenv("APALACHE_GO")
	if scalaExecutable == "" || goExecutable == "" {
		t.Skip("set APALACHE_SCALA and APALACHE_GO to run live differential tests")
	}
	data, err := os.ReadFile("testdata/oracle/safety.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest oraclectl.Manifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, test := range manifest.Cases {
		t.Run(test.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			result, err := oraclectl.RunDifferential(ctx, scalaExecutable, goExecutable, test)
			if err != nil {
				t.Fatalf("%v\nScala stdout:\n%s\nScala stderr:\n%s\nGo stdout:\n%s\nGo stderr:\n%s",
					err, result.Scala.Stdout, result.Scala.Stderr, result.Go.Stdout, result.Go.Stderr)
			}
		})
	}
}
