package networking

import (
	"github.com/vishvananda/netlink"
	"github.com/containernetworking/cni/pkg/types/current"
	"fmt"
	"os"
	"syscall"
	"net"
)

func CreateRoute(dev netlink.Link, result *current.Result) error {

	gw, err := netlink.AddrList(dev, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("error looking up up IP for %s: %v", dev.Attrs().Name, err)
	}
	dst := netlink.NewIPNet(result.IPs[0].Address.IP)
	// Make sure that we exactly match the IP
	dst.Mask = net.IPv4Mask(255, 255, 255, 255)
	route := &netlink.Route{
		Dst: dst,
		Gw:  gw[0].IP,
	}

	err = netlink.RouteReplace(route)
	if err != nil && !os.IsExist(err) {
		return fmt.Errorf("error creating route %v: %v", route, err)
	}
	return nil
}

func DeleteRoute(dev netlink.Link, ip net.IP) error {

	gw, err := netlink.AddrList(dev, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("error looking up up IP for %s: %v", dev.Attrs().Name, err)
	}
	dst := netlink.NewIPNet(ip)
	// Make sure that we exactly match the IP
	dst.Mask = net.IPv4Mask(255, 255, 255, 255)
	// remove route
	route := &netlink.Route{
		Dst: dst,
		Gw:  gw[0].IP,
	}
	err = netlink.RouteDel(route)
	// In case that the route does not exist, I got an ESRCH returned
	// TODO should that be added to os.IsNotExist?
	if err != nil && !os.IsNotExist(err) && underlyingError(err) != syscall.ESRCH {
		return fmt.Errorf("error deleting route %v: %v", route, err)
	}
	return nil
}

// underlyingError returns the underlying error for known os error types.
func underlyingError(err error) error {
	switch err := err.(type) {
	case *os.PathError:
		return err.Err
	case *os.LinkError:
		return err.Err
	case *os.SyscallError:
		return err.Err
	}
	return err
}