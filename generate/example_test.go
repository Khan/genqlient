package generate

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGenerateExample(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller non-ok")
	}

	repoRoot := filepath.Dir(filepath.Dir(thisFile))
	configFilename := filepath.Join(repoRoot, "example/genql.yaml")
	config, err := ReadAndValidateConfig(configFilename)
	if err != nil {
		t.Fatal(err)
	}

	code, err := Generate(config)
	if err != nil {
		t.Fatal(err)
	}

	expectedCode, err := ioutil.ReadFile(config.Generated)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(code, expectedCode) {
		t.Errorf(
			"diffs to generated code:\n---actual---\n%v\n---expected---\n%v",
			string(code), string(expectedCode))
	}
}
