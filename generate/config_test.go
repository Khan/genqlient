package generate

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindCfg(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	cases := map[string]struct {
		startDir    string
		expectedCfg string
		expectedErr error
	}{
		"yaml in parent directory": {
			startDir:    cwd + "/testdata/find-config/parent/child",
			expectedCfg: cwd + "/testdata/find-config/parent/genqlient.yaml",
		},
		"yaml in current directory": {
			startDir:    cwd + "/testdata/find-config/current",
			expectedCfg: cwd + "/testdata/find-config/current/genqlient.yaml",
		},
		"no yaml": {
			startDir:    cwd + "/testdata/find-config/none/child",
			expectedErr: os.ErrNotExist,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				require.NoError(t, os.Chdir(cwd), "Test cleanup failed")
			}()

			err = os.Chdir(tc.startDir)
			require.NoError(t, err)

			path, err := findCfg()
			assert.Equal(t, tc.expectedCfg, path)
			assert.Equal(t, tc.expectedErr, err)
		})
	}
}

func TestFindCfgInDir(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	cases := map[string]struct {
		startDir string
		found    bool
	}{
		"yaml": {
			startDir: cwd + "/testdata/find-config/filenames/yaml",
			found:    true,
		},
		"yml": {
			startDir: cwd + "/testdata/find-config/filenames/yml",
			found:    true,
		},
		".yaml": {
			startDir: cwd + "/testdata/find-config/filenames/dotyaml",
			found:    true,
		},
		".yml": {
			startDir: cwd + "/testdata/find-config/filenames/dotyml",
			found:    true,
		},
		"none": {
			startDir: cwd + "/testdata/find-config/filenames/none",
			found:    false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			path := findCfgInDir(tc.startDir)
			if tc.found {
				assert.NotEmpty(t, path)
			} else {
				assert.Empty(t, path)
			}
		})
	}
}

func TestAbsoluteAndRelativePathsInConfigFiles(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)

	config, err := ReadAndValidateConfig(
		cwd + "/testdata/find-config/current/genqlient.yaml")
	require.NoError(t, err)

	require.Equal(t, 1, len(config.Schema))
	require.Equal(
		t,
		cwd+"/testdata/find-config/current/schema.graphql",
		config.Schema[0],
	)
	require.Equal(t, 1, len(config.Operations))
	require.Equal(t, "/tmp/genqlient.graphql", config.Operations[0])
}
