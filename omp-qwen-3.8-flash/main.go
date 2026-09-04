// vsphere-inventory reports vCenter virtualization inventory (virtual
// machines, datastores, virtual switches) as plain-text tables.
package main

import "github.com/ldh/vsphere-inventory/cmd"

func main() {
	cmd.Execute()
}
