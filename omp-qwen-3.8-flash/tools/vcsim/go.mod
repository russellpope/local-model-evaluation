module localtools/vcsim

// Vcsim driver vendored from github.com/vmware/govmomi/vcsim
// (v0.0.0-20260821034451-81608f9b9725, the commit tagged v0.56.0), with the
// upstream `replace github.com/vmware/govmomi => ../` directive swapped for a
// pinned require so the binary builds without cloning the govmomi repo. The
// nested module form cannot otherwise be `go run`'d by consumers: Go rejects
// modules containing replace directives when fetched as a dependency.

go 1.25.0

require github.com/vmware/govmomi v0.56.0

require (
	github.com/google/uuid v1.6.0
	golang.org/x/text v0.41.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
