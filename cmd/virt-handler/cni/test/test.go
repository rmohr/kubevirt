package main

import (
	"github.com/spf13/pflag"
	"net"
	"log"
	"github.com/mdlayher/raw"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"bytes"
	"github.com/mdlayher/ethernet"
)

var EthernetBroadcastAddr = raw.Addr{
	HardwareAddr: ethernet.Broadcast,
}

func main() {
	var iface string
	var oface string
	pflag.StringVarP(&iface, "iface", "i", "mybridge1", "Device to listen for ethernet frames")
	pflag.StringVarP(&oface, "oface", "o", "kubevirt0", "Device to write ethernet frames")
	pflag.Parse()

	inHandle := newConn(iface)
	inHandle.SetPromiscuous(true)
	defer inHandle.Close()

	outHandle := newConn(oface)

	src := gopacket.NewPacketSource(inHandle, layers.LayerTypeEthernet)
	in := src.Packets()


	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	for {
		var packet gopacket.Packet
		select {
		case packet = <-in:

			// Filter out packets I sent
			ethLayer := packet.Layer(layers.LayerTypeEthernet)

			if ethLayer == nil {
				continue
			}

			eth := ethLayer.(*layers.Ethernet)

			if bytes.Equal([]byte(inHandle.Address()), eth.SrcMAC) {
				// This is a packet I sent.
				continue
			}

			// Check if we deal with a dhcp server response
			udpLayer := packet.Layer(layers.LayerTypeUDP)
			if udpLayer == nil {
				continue
			}
			udp := udpLayer.(*layers.UDP)
			udp.SetNetworkLayerForChecksum(packet.NetworkLayer())


			if  udp.SrcPort != 67 {
				// Not from a dhcp server
				continue
			}

			// Check if we are dealing with a DHCP response
			dhcpLayer := packet.Layer(layers.LayerTypeDHCPv4)
			if dhcpLayer == nil {
				continue
			}

			dhcp := dhcpLayer.(*layers.DHCPv4)

			// Log what we found
			log.Printf("Type: %v", getOptionType(dhcp.Options))
			log.Printf("Operation: %v", dhcp.Operation)
			log.Printf("Client IP: %v", dhcp.ClientIP)
			log.Printf("Your IP: %v", dhcp.YourClientIP)
			log.Printf("Client HWADDR: %v", dhcp.ClientHWAddr)


			// Re-send the packet, so that it can be rerouted to the VM
			// TODO check if just replacing the package is good enough to avoid loop processing
			eth.SrcMAC = []byte(inHandle.Address())

			if err := gopacket.SerializePacket(buf, opts, packet); err != nil {
				log.Fatalf("Failed to serialize packet: %v", err)
			}

			if err := outHandle.WritePacketData(buf.Bytes()); err != nil {
				log.Fatalf("Failed to write packet: %v", err)
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


func newConn(iface string) *Handle {
	i, err := net.InterfaceByName(iface)
	if err != nil {
		log.Fatalf("Error %v when searching for device %s", err, i)
	}
	conn, err := raw.ListenPacket(i, 0x3)
	if err != nil {
		log.Fatalf("Failed to open socket to device %s: %v",iface,  err)
	}

	// Accept frames up to interface's MTU in size.
	return &Handle{
		buf: make([]byte, i.MTU),
		conn: conn,
		index: i.Index,
		addr: i.HardwareAddr,
	}
}