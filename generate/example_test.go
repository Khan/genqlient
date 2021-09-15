package generate_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Khan/genqlient/internal/integration"
)

func TestGenerateExample(t *testing.T) {
	integration.RunGenerateTest(t, "example/genqlient.yaml")
}

func TestRunExample(t *testing.T) {
	if _, ok := os.LookupEnv("GITHUB_TOKEN"); !ok {
		t.Skip("requires GITHUB_TOKEN to be set")
	}

	cmd := exec.Command("go", "run", "./example", "benjaminjkraft")
	cmd.Dir = integration.RepoRoot(t)
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
