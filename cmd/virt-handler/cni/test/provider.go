package main

import (
	"net"
	"log"
	"github.com/spf13/pflag"
	"kubevirt.io/kubevirt/pkg/networking"
)

func main() {

	parentIf := pflag.String("parent", "parentBridge", "parent device")
	targetIf := pflag.String("target", "targetBridge", "target device")
	delete := pflag.BoolP("delete", "d",  false, "Remove everything and quit")
	pflag.Parse()

	tc, err := networking.NewTC(*parentIf, *targetIf)
	mac, _ := net.ParseMAC("6e:3f:a2:cf:f8:12")

	if !*delete {
		if err != nil {
			log.Fatalf("Could not open devices %v", err)
		}
		err = tc.EnsureIngressQDisc()
		if err != nil {
			log.Fatalf("Could not ensure presence of qdisc: %v", err)
		}

		err = tc.AddMangledPacketsFilter()
		if err != nil {
			log.Fatalf("Could not add mangled packets filter: %v", err)
		}

		err = tc.Add(mac)
		if err != nil {
			log.Fatalf("Could not add VM filter: %v", err)
		}
	} else {
		err = tc.Del(mac)
		if err != nil {
			log.Fatalf("Could not delete VM filter: %v", err)
		}
		tc.RemoveMangledPacketsFilter()
		if err != nil {
			log.Fatalf("Could not remove VM mangle filter: %v", err)
		}
	}
}
