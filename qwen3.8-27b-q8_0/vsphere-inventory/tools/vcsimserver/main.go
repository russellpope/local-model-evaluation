// Command vcsimserver starts a minimal in-process vcsim (govmomi simulator)
// vCenter endpoint for local verification of the vsphere-inventory CLI.
//
// It is a separate Go module so that the CLI module's dependency set stays
// limited to govmomi, cobra and viper. It serves plain HTTP (no TLS) and
// prints the SDK URL on the first line of stdout once the server is ready.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	var (
		listen     = flag.String("l", "127.0.0.1:0", "listen address (host:port; port 0 picks a free port)")
		datacenter = flag.Int("dc", 1, "number of datacenters")
		datastore  = flag.Int("ds", 3, "number of local datastores")
		machine    = flag.Int("vm", 2, "number of virtual machines per resource pool")
		portgroup  = flag.Int("pg", 3, "number of distributed port groups")
	)
	flag.Parse()

	model := simulator.VPX()
	model.Datacenter = *datacenter
	model.Datastore = *datastore
	model.Machine = *machine
	model.Portgroup = *portgroup

	if err := model.Create(); err != nil {
		log.Fatalf("create model: %v", err)
	}
	defer model.Remove()

	// Plain HTTP: leave Service.TLS nil so the URL scheme stays "http".
	model.Service.Listen = &url.URL{Host: *listen}

	s := model.Service.NewServer()
	defer s.Close()

	// First line on stdout is the ready URL so a wrapper can capture it.
	fmt.Println(s.URL.String())
	fmt.Fprintln(os.Stderr, "vcsimserver ready; press Ctrl-C to stop")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}
