package main

import (
	"context"
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
	var (
		vmCount      int
		dsCount      int
		pgCount      int
		hostCount    int
		clusterCount int
		clusterHosts int
		dcCount      int
		standalone   bool
		port         int
	)

	flag.IntVar(&vmCount, "vm", 2, "number of VMs per resource pool")
	flag.IntVar(&dsCount, "ds", 1, "number of datastores per datacenter")
	flag.IntVar(&pgCount, "pg", 1, "number of distributed virtual port groups per datacenter")
	flag.IntVar(&hostCount, "host", 0, "number of standalone hosts per datacenter")
	flag.IntVar(&clusterCount, "cluster", 0, "number of clusters per datacenter")
	flag.IntVar(&clusterHosts, "clusterHost", 0, "number of hosts per cluster")
	flag.IntVar(&dcCount, "dc", 1, "number of datacenters")
	flag.BoolVar(&standalone, "esx", false, "run as standalone ESX (not vCenter)")
	flag.IntVar(&port, "port", 0, "port to listen on (0 = random)")

	flag.Parse()

	var model *simulator.Model
	if standalone {
		model = simulator.ESX()
	} else {
		model = simulator.VPX()
	}

	model.Datacenter = dcCount
	model.Datastore = dsCount
	model.Machine = vmCount
	model.Portgroup = pgCount
	model.Host = hostCount
	if clusterCount > 0 {
		model.Cluster = clusterCount
	}
	if clusterHosts > 0 {
		model.ClusterHost = clusterHosts
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := model.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating simulator: %v\n", err)
		os.Exit(1)
	}
	defer model.Remove()

	model.Service.TLS = new(tls.Config)
	model.Service.RegisterEndpoints = true

	if port > 0 {
		model.Service.Listen = &url.URL{
			Scheme: "https",
			Host:   fmt.Sprintf("127.0.0.1:%d", port),
		}
	}

	s := model.Service.NewServer()
	defer s.Close()

	fmt.Printf("vcsim is listening on %s\n", s.URL.String())
	fmt.Printf("username: user\n")
	fmt.Printf("password: pass\n")
	fmt.Printf("insecure: true\n")
	fmt.Println()

	<-ctx.Done()
	fmt.Println("\nShutting down vcsim...")
}
