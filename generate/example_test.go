package generate

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func getRepoRoot(t *testing.T) string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller non-ok")
	}

	return filepath.Dir(filepath.Dir(thisFile))
}

func TestGenerateExample(t *testing.T) {
	configFilename := filepath.Join(getRepoRoot(t), "example/genqlient.yaml")
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

func TestRunExample(t *testing.T) {
	if _, ok := os.LookupEnv("GITHUB_TOKEN"); !ok {
		t.Skip("requires GITHUB_TOKEN to be set")
	}

	cmd := exec.Command("go", "run", "./example/cmd/example", "benjaminjkraft")
	cmd.Dir = getRepoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(err)
	}

	got := strings.TrimSpace(string(out))
	want := "benjaminjkraft is Ben Kraft created on 2009-08-03"
	if got != want {
		t.Errorf("output incorrect\ngot:\n%s\nwant:\n%s", got, want)
	}
}
