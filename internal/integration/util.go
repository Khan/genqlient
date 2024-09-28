package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Khan/genqlient/generate"
	"github.com/Khan/genqlient/graphql"
)

// RepoRoot returns the root of the genqlient repository,
func RepoRoot(t *testing.T) string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller non-ok")
	}

	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	if _, err := os.Stat(filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(fmt.Errorf("doesn't look like repo root: %v", err))
	}
	return root
}

// RunGenerateTest checks that running genqlient with the given
// repo-root-relative config file would not produce any changes to the
// checked-in files.
func RunGenerateTest(t *testing.T, relConfigFilename string) {
	configFilename := filepath.Join(RepoRoot(t), relConfigFilename)
	config, err := generate.ReadAndValidateConfig(configFilename)
	if err != nil {
		t.Fatal(err)
	}

	generated, err := generate.Generate(config)
	if err != nil {
		t.Fatal(err)
	}

	for filename, content := range generated {
		expectedContent, err := os.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(content, expectedContent) {
			t.Errorf("mismatch in %s", filename)
			if testing.Verbose() {
				t.Errorf("got:\n%s\nwant:\n%s\n", content, expectedContent)
			}
			if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
				err = os.WriteFile(filename, content, 0o644)
				if err != nil {
					t.Errorf("unable to update generated file %s: %v", filename, err)
				} else {
					t.Errorf("updated generated file for %s", filename)
				}
			}
		}
	}
}

// Used for a binding in genqlient.yaml.
//
// This is here rather than in testutil to test the case where the generated
// code and the bound type are in the same package.
type MyGreatScalar string

func forwardData[T any](dataChan_ chan graphql.WsResponse[T], jsonRawMsg json.RawMessage) error {
	var gqlResp graphql.Response
	var wsResp graphql.WsResponse[T]
	err := json.Unmarshal(jsonRawMsg, &gqlResp)
	if err != nil {
		return err
	}
	if len(gqlResp.Errors) == 0 {
		err = json.Unmarshal(jsonRawMsg, &wsResp)
		if err != nil {
			return err
		}
	} else {
		wsResp.Errors = gqlResp.Errors
	}
	dataChan_ <- wsResp
	return nil
}
