//go:build ignore

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
	vmCount := flag.Int("vm", 4, "virtual machines per resource pool")
	dsCount := flag.Int("ds", 1, "datastores")
	pgCount := flag.Int("pg", 0, "distributed portgroups per datacenter")
	listen := flag.String("l", "127.0.0.1:8989", "listen address")
	flag.Parse()

	model := simulator.VPX()
	model.Machine = *vmCount
	model.Datastore = *dsCount
	model.Portgroup = *pgCount

	if err := model.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "create simulator model: %v\n", err)
		os.Exit(1)
	}
	defer model.Remove()

	model.Service.Listen = &url.URL{
		Host: *listen,
		User: url.UserPassword("user", "pass"),
	}
	model.Service.TLS = &tls.Config{}

	server := model.Service.NewServer()
	defer server.Close()

	fmt.Fprintf(os.Stderr, "vcsim listening at %s\n", server.URL.String())

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	<-done
}
