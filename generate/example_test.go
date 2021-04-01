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
	configFilename := filepath.Join(repoRoot, "example/genqlient.yaml")
	config, err := ReadAndValidateConfig(configFilename)
	if err != nil {
		t.Fatal(err)
	}

	generated, err := Generate(config)
	if err != nil {
		t.Fatal(err)
	}

	for filename, content := range generated {
		expectedContent, err := ioutil.ReadFile(filename)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(content, expectedContent) {
			t.Errorf(
				"diffs to %v:\n---actual---\n%v\n---expected---\n%v",
				filename, string(content), string(expectedContent))
		}
	}
}
