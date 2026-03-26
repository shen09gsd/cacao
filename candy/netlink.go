package candy

import (
	"net"

	"github.com/lanthora/cacao/logger"
	"github.com/vishvananda/netlink"
)

// AddKernelRoute adds a route to the Linux kernel routing table
func AddKernelRoute(dst *net.IPNet, gw net.IP, iface *net.Interface) error {
	route := &netlink.Route{
		Dst:       dst,
		Gw:        gw,
		Interface: iface,
	}
	if err := netlink.RouteReplace(route); err != nil {
		logger.Debug("add kernel route failed: %v", err)
		return err
	}
	return nil
}

// DelKernelRoute deletes a route from the Linux kernel routing table
func DelKernelRoute(dst *net.IPNet, gw net.IP, iface *net.Interface) error {
	route := &netlink.Route{
		Dst:       dst,
		Gw:        gw,
		Interface: iface,
	}
	if err := netlink.RouteDel(route); err != nil {
		logger.Debug("del kernel route failed: %v", err)
		return err
	}
	return nil
}

// FlushKernelRoutes removes all routes associated with a specific interface
func FlushKernelRoutes(ifaceName string) error {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		logger.Debug("list routes failed: %v", err)
		return err
	}
	for _, r := range routes {
		if r.Interface != nil && r.Interface.Name == ifaceName {
			if err := netlink.RouteDel(&r); err != nil {
				logger.Debug("flush route failed: %v", err)
			}
		}
	}
	return nil
}

// GetInterfaceByName returns a network interface by its name
func GetInterfaceByName(name string) (*net.Interface, error) {
	return net.InterfaceByName(name)
}
