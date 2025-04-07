package itest

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"

	"github.com/stretchr/testify/require"
)

type tempDirBase struct {
	tempDir    string
	tempDirSeq uint64
}

type tempDirBaseKey struct{}

func withTempDirBase(ctx context.Context, td *tempDirBase) context.Context {
	return context.WithValue(ctx, tempDirBaseKey{}, td)
}

// TempDir returns a temporary directory for the test to use.
// The directory is automatically removed when the test and
// all its subtests complete.
// Each subsequent call to t.TempDir returns a unique directory;
// if the directory creation fails, TempDir terminates the test by calling Fatal.
func TempDir(ctx context.Context) string {
	t := getT(ctx)
	if td, ok := ctx.Value(tempDirBaseKey{}).(*tempDirBase); ok {
		seq := atomic.AddUint64(&td.tempDirSeq, 1)
		dir := fmt.Sprintf("%s%c%03d", td.tempDir, os.PathSeparator, seq)
		require.NoError(t, os.Mkdir(dir, 0o777))
		return dir
	}
	return t.TempDir()
}
