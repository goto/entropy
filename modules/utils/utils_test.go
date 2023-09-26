package utils

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"gotest.tools/assert"
)

func TestCloneAndMergeMaps(t *testing.T) {
	m1 := map[string]string{"a1": "b1", "a2": "b2"}
	m2 := map[string]string{"a3": "b3"}
	m3 := CloneAndMergeMaps(m1, m2)
	assert.Assert(t, cmp.Equal(map[string]string{"a1": "b1", "a2": "b2", "a3": "b3"}, m3))
}

func TestSafeName(t *testing.T) {
	sName := SafeName("testing-1232-end", "-name", 50)
	fmt.Println(sName)
	assert.Equal(t, "testing-1232-end-name", sName)
}
