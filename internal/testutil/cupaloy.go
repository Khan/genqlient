package testutil

import "github.com/bradleyjkemp/cupaloy/v2"

var Cupaloy = cupaloy.New(cupaloy.SnapshotSubdirectory("testdata/snapshots"))
