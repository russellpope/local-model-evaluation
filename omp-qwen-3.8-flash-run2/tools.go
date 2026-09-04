//go:build tools

// Package tools pins command dependencies used by the build/test tooling so
// `go mod tidy` keeps them and `go run github.com/vmware/govmomi/vcsim`
// always resolves to the same govmomi version the binary is built against.
package tools

import _ "github.com/vmware/govmomi/vcsim"
