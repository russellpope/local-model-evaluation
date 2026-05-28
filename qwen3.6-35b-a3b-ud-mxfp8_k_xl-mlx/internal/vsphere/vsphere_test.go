package vsphere_test

import (
	"testing"
)

func TestSkipIntegration(t *testing.T) {
	t.Skip("Integration tests require live vcsim setup - use 'make verify' instead")
}
