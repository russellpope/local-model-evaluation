package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 4
	model.Datastore = 2
	model.Portgroup = 2

	if err := model.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create model: %v\n", err)
		os.Exit(1)
	}
	defer model.Remove()

	s := model.Service.NewServer()
	defer s.Close()

	fmt.Printf("vcsim listening on %s\n", s.URL)

	// Start a simple HTTP server on port 8989 that just responds with OK
	// This allows make verify to check readiness
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})
		if err := http.ListenAndServe(":8989", nil); err != nil {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Block until killed
	select {}
}
