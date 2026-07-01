package main

import (
	"fmt"
	"os"
	"time"

	"github.com/vmware/govmomi/simulator"
)

func main() {
	model := simulator.VPX()
	model.Host = 1
	model.Machine = 2
	model.Datastore = 2

	if err := model.Create(); err != nil {
		fmt.Fprintf(os.Stderr, "create simulator model: %v\n", err)
		os.Exit(1)
	}
	defer model.Remove()

	s := model.Service.NewServer()
	defer s.Close()

	url := fmt.Sprintf("%s://%s/sdk", s.URL.Scheme, s.URL.Host)
	fmt.Println(url)
	os.Stdout.Sync()

	for {
		time.Sleep(1 * time.Second)
	}
}
