package generate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Khan/genqlient/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	findConfigDir    = "testdata/find-config"
	validConfigDir   = "testdata/valid-config"
	invalidConfigDir = "testdata/invalid-config"
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
			startDir:    filepath.Join(cwd, findConfigDir, "parent", "child"),
			expectedCfg: filepath.Join(cwd, findConfigDir, "parent", "genqlient.yaml"),
		},
		"yaml in current directory": {
			startDir:    filepath.Join(cwd, findConfigDir, "current"),
			expectedCfg: filepath.Join(cwd, findConfigDir, "current", "genqlient.yaml"),
		},
		"no yaml": {
			startDir:    filepath.Join(cwd, findConfigDir, "none", "child"),
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
			startDir: filepath.Join(cwd, findConfigDir, "filenames", "yaml"),
			found:    true,
		},
		"yml": {
			startDir: filepath.Join(cwd, findConfigDir, "filenames", "yml"),
			found:    true,
		},
		".yaml": {
			startDir: filepath.Join(cwd, findConfigDir, "filenames", "dotyaml"),
			found:    true,
		},
		".yml": {
			startDir: filepath.Join(cwd, findConfigDir, "filenames", "dotyml"),
			found:    true,
		},
		"none": {
			startDir: filepath.Join(cwd, findConfigDir, "filenames", "none"),
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
		filepath.Join(cwd, findConfigDir, "current", "genqlient.yaml"))
	require.NoError(t, err)

	require.Equal(t, 1, len(config.Schema))
	require.Equal(
		t,
		filepath.Join(cwd, findConfigDir, "current", "schema.graphql"),
		config.Schema[0],
	)
	require.Equal(t, 1, len(config.Operations))
	require.Equal(t, "/tmp/genqlient.graphql", config.Operations[0])
}

func testAllSnapshots(
	t *testing.T,
	dir string,
	testfunc func(t *testing.T, filename string),
) {
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, file := range files {
		name := file.Name()
		if name[0] == '.' {
			continue // editor backup files, etc.
		}
		t.Run(name, func(t *testing.T) {
			filename := filepath.Join(dir, file.Name())
			testfunc(t, filename)
		})
	}
}

func TestValidConfigs(t *testing.T) {
	testAllSnapshots(t, validConfigDir, func(t *testing.T, filename string) {
		config, err := ReadAndValidateConfig(filename)
		require.NoError(t, err)
		testutil.Cupaloy.SnapshotT(t, config)
	})
}

func TestInvalidConfigs(t *testing.T) {
	testAllSnapshots(t, invalidConfigDir, func(t *testing.T, filename string) {
		_, err := ReadAndValidateConfig(filename)
		require.Error(t, err)
		testutil.Cupaloy.SnapshotT(t, err.Error())
	})
}
