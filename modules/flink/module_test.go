package flink

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDriverFactory_EmptyConfig(t *testing.T) {
	t.Parallel()

	for _, conf := range [][]byte{nil, []byte("null"), []byte("{}")} {
		drv, err := Module.DriverFactory(conf)
		require.NoError(t, err)
		require.NotNil(t, drv)

		fd, ok := drv.(*flinkDriver)
		require.True(t, ok)
		assert.NotNil(t, fd)
	}
}
