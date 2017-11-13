package networking

import (
	"net"
	"github.com/mdlayher/raw"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"bytes"
	"github.com/mdlayher/ethernet"
	"fmt"
	"kubevirt.io/kubevirt/pkg/log"
	"github.com/vishvananda/netlink"
)

var EthernetBroadcastAddr = raw.Addr{
	HardwareAddr: ethernet.Broadcast,
}

type DHCPAck struct {
	IP net.IP
	MAC net.HardwareAddr
}

func RunDHCPSniffer(iface, oface string, stopChan chan struct{}, observed chan DHCPAck) error {

	// Open a connection to iface and set it to promiscuous mode
	inHandle, err := newConn(iface)
	if err != nil {
		return fmt.Errorf("Error opening connection to %s: %v", iface, err)
	}
	defer inHandle.Close()
	err = inHandle.SetPromiscuous(true)
	if err != nil {
		return fmt.Errorf("Error setting device %s to promiscuous mode: %v", iface, err)
	}

	// Ensure that oface is up
	ofaceDev, err := netlink.LinkByName(oface)
	if err != nil {
		return fmt.Errorf("Error getting device %s: %v", oface, err)
	}
	err = netlink.LinkSetUp(ofaceDev)
	if err != nil {
		return fmt.Errorf("Error sett device %s to state up: %v", oface, err)
	}

	// Open a connection to oface
	outHandle, err := newConn(oface)
	if err != nil {
		return fmt.Errorf("Error opening connection to %s: %v", oface, err)
	}
	defer outHandle.Close()

	src := gopacket.NewPacketSource(inHandle, layers.LayerTypeEthernet)
	in := src.Packets()

	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	for {
		var packet gopacket.Packet
		var open bool
		select {
		case <- stopChan:
			return nil
		case packet, open = <-in:

			if !open{
				return nil
			}

			// Filter out packets I sent
			ethLayer := packet.Layer(layers.LayerTypeEthernet)

			if ethLayer == nil {
				continue
			}

			eth := ethLayer.(*layers.Ethernet)

			if bytes.Equal([]byte(inHandle.Address()), eth.SrcMAC) {
				// This is a packet I sent.
				log.Log.V(5).Infof("Ignoring packet which I sent")
				continue
			}

			// Check if we deal with a dhcp server response
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpLayer == nil {
				continue
			}
			udp := udpLayer.(*layers.UDP)
			udp.SetNetworkLayerForChecksum(packet.NetworkLayer())


			if  udp.DstPort != 68 {
				// Not for a dhcp client
				continue
			}

			// Check if we are dealing with a DHCP response
			dhcpLayer := packet.Layer(layers.LayerTypeDHCPv4)
			if dhcpLayer == nil {
				continue
			}

			dhcp := dhcpLayer.(*layers.DHCPv4)

			// Log what we found
			log.Log.V(5).Infof("Type: %v", getOptionType(dhcp.Options))
			log.Log.V(5).Infof("Operation: %v", dhcp.Operation)
			log.Log.V(5).Infof("Client IP: %v", dhcp.ClientIP)
			log.Log.V(5).Infof("Your IP: %v", dhcp.YourClientIP)
			log.Log.V(5).Infof("Client HWADDR: %v", dhcp.ClientHWAddr)

			// If we have a DHCP ack, inform us about the discovered details
			if getOptionType(dhcp.Options) == layers.DHCPMsgTypeAck {
				observed <- DHCPAck{
					IP: dhcp.YourClientIP,
					MAC: dhcp.ClientHWAddr,
				}
			}

			// Re-send the packet, so that it can be rerouted to the VM
			eth.SrcMAC = []byte(inHandle.Address())

			if err := gopacket.SerializePacket(buf, opts, packet); err != nil {
				log.Log.Reason(err).Error("Failed to serialize packet")
				continue
			}

			if err := outHandle.WritePacketData(buf.Bytes()); err != nil {
				return fmt.Errorf("Error writing to interface %s: %v", oface, err)
			}
		}
	}
}

type Handle struct {
	conn *raw.Conn
	buf []byte
	index int
	addr net.HardwareAddr
}

func (h *Handle) ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	n, _, err := h.conn.ReadFrom(h.buf)
	if err != nil {
		return
	}
	ci.InterfaceIndex = h.index
	ci.CaptureLength = n
	ci.Length = n
	data = h.buf[0:n]
	return
}

func (p *Handle) WritePacketData(data []byte) (err error) {
	_, err = p.conn.WriteTo(data, &EthernetBroadcastAddr)
	return err
}

func (p *Handle) Close() {
	p.conn.Close()
}

func (p *Handle) SetPromiscuous(set bool) error {
	return p.conn.SetPromiscuous(set)
}

func (p *Handle) Address() net.HardwareAddr {
	return p.addr
}

func getOptionType(options layers.DHCPOptions) layers.DHCPMsgType {
	for _, o := range options {
		if o.Type == layers.DHCPOptMessageType {
			return layers.DHCPMsgType(o.Data[0])
		}
	}

	return layers.DHCPMsgTypeUnspecified
}


func newConn(iface string) (*Handle, error) {
	i, err := net.InterfaceByName(iface)
	if err != nil {
		return nil, fmt.Errorf("Error %v when searching for device %s", err, i)
	}
	conn, err := raw.ListenPacket(i, 0x3)
	if err != nil {
		return nil, fmt.Errorf("Failed to open socket to device %s: %v",iface,  err)
	}

	// Accept frames up to interface's MTU in size.
	return &Handle{
		buf: make([]byte, i.MTU),
		conn: conn,
		index: i.Index,
		addr: i.HardwareAddr,
	}, nil
}