// Command vcsim starts govmomi's bundled in-memory vCenter simulator on a
// fixed endpoint for local development and the `make verify` self-check.
//
// In govmomi v0.54.0 the upstream vcsim command lives in a separate module, so
// this launcher wraps the simulator package that ships inside the govmomi
// version this project already depends on — guaranteeing it runs at exactly
// that version with no extra dependency. It mirrors the upstream defaults:
// endpoint https://127.0.0.1:8989/sdk, username "user", password "pass",
// self-signed TLS (connect with insecure=true).
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	vm := flag.Int("vm", 2, "number of virtual machines")
	ds := flag.Int("ds", 1, "number of datastores")
	pg := flag.Int("pg", 1, "number of distributed port groups")
	listen := flag.String("l", "127.0.0.1:8989", "listen address (host:port)")
	flag.Parse()

	model := simulator.VPX()
	model.Machine = *vm
	model.Datastore = *ds
	model.Portgroup = *pg

	if err := model.Create(); err != nil {
		fmt.Fprintln(os.Stderr, "vcsim: create model:", err)
		os.Exit(1)
	}
	defer model.Remove()

	model.Service.TLS = new(tls.Config)
	model.Service.Listen = &url.URL{Host: *listen}

	server := model.Service.NewServer()
	defer server.Close()

	fmt.Printf("vcsim listening at %s\n", server.URL)
	fmt.Println("export VSPHERE_URL=https://" + *listen + "/sdk VSPHERE_USERNAME=user VSPHERE_PASSWORD=pass VSPHERE_INSECURE=true")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	fmt.Println("\nvcsim: shutting down")
}
