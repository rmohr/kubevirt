package pkg

import (
	"encoding/binary"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"net"
	"os"
)

func NewTC(ifParent, ifTarget string) (*TC, error) {
	parentLink, err := netlink.LinkByName(ifParent)
	if err != nil {
		return nil, err
	}
	targetLink, err := netlink.LinkByName(ifTarget)
	if err != nil {
		return nil, err
	}

	return &TC{
		ifParent: parentLink,
		ifTarget: targetLink,
	}, nil
}

type TC struct {
	ifParent netlink.Link
	ifTarget netlink.Link
}

type Args struct {
	ID  string
	MAC net.HardwareAddr
}

type Reply struct {
	ID  string
	MAC net.HardwareAddr
	IP  net.IP
}

func (t *TC) Add(mac net.HardwareAddr) error {
	return netlink.FilterAdd(t.vmPacketFilter(mac))
}

func (t *TC) Del(mac net.HardwareAddr) error {
	return netlink.FilterDel(t.vmPacketFilter(mac))
}

func (t *TC) vmPacketFilter(mac net.HardwareAddr) netlink.Filter {
	lo, hi := parseMAC(mac)

	selectors := &netlink.TcU32Sel{
		Flags: netlink.TC_U32_TERMINAL,
		Keys: []netlink.TcU32Key{
			// match destination mac
			{
				Off:  -16,
				Mask: 0x0000ffff,
				Val:  hi, // first part of mac
			},
			{
				Off:  -12,
				Mask: 0xffffffff,
				Val:  lo, // second part of mac
			},
			// match dhcp target port
			{
				Off:  20,
				Val:  0x00000044, // port 68
				Mask: 0x0000ffff,
			},
		},
	}

	// Redirect all matching packets to the target interface
	// Priority is 2, to allow not routing re-sent packets by matching a filter with priority 1
	return &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: t.ifParent.Attrs().Index,
			Parent:    netlink.MakeHandle(0xffff, 0),
			Priority:  2,
			Protocol:  unix.ETH_P_IP,
		},
		Actions: []netlink.Action{netlink.NewMirredAction(t.ifTarget.Attrs().Index)},
		Sel:     selectors,
	}
}

func (t *TC) mangledPacketFilter(mac net.HardwareAddr) netlink.Filter {
	lo, hi := parseMAC(mac)

	selectors := &netlink.TcU32Sel{
		Flags: netlink.TC_U32_TERMINAL,
		Keys: []netlink.TcU32Key{
			// match source mac
			{
				Off:  -8,
				Mask: 0x0000ffff,
				Val:  hi, // first part of mac
			},
			{
				Off:  -4,
				Mask: 0xffffffff,
				Val:  lo, // second part of mac
			},
			// match dhcp target port
			{
				Off:  20,
				Val:  0x00000044, // port 68
				Mask: 0x0000ffff,
			},
		},
	}

	// If we have a source MAC match, we match this packet, to break a redirect loop
	return &netlink.U32{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: t.ifParent.Attrs().Index,
			Parent:    netlink.MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_IP,
		},
		Actions: []netlink.Action{&netlink.GenericAction{
			ActionAttrs: netlink.ActionAttrs{
				Action: netlink.TC_ACT_OK,
			} ,
		}},
		Sel:     selectors,
	}
}

func (t *TC) EnsureIngressQDisc() error {
	qdisc := &netlink.Ingress{
		QdiscAttrs: netlink.QdiscAttrs{
			Parent:    netlink.HANDLE_INGRESS,
			LinkIndex: t.ifParent.Attrs().Index,
			Handle:    netlink.MakeHandle(0xffff, 0),
		},
	}
	err := netlink.QdiscAdd(qdisc)
	if err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

func (t *TC)AddMangledPacketsFilter() error {
	filter := t.mangledPacketFilter(t.ifTarget.Attrs().HardwareAddr)
	if err := netlink.FilterAdd(filter); err != nil {
		return err
	}
	return nil
}

func (t *TC)RemoveMangledPacketsFilter() error {
	filter := t.mangledPacketFilter(t.ifTarget.Attrs().HardwareAddr)
	if err := netlink.FilterDel(filter); err != nil {
		return err
	}
	return nil
}

func parseMAC(mac net.HardwareAddr) (lo, hi uint32) {
	first := binary.BigEndian.Uint32(mac[0:4])
	second := binary.BigEndian.Uint32(mac[2:6])
	return second, (first >> 16) & 0x0000ffff
}
