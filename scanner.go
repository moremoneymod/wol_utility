package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

func scan(ctx context.Context, cidr string, adapterNum int, ch chan string) error {
	devices, err := getNetworkAdapters()
	if err != nil {
		return err
	}
	device := devices[adapterNum]
	if len(device.Addresses) == 0 || device.Addresses[0].IP.To4() == nil {
		return fmt.Errorf("выбран неправильный адаптер")
	}
	fmt.Println(device)
	handle, err := pcap.OpenLive(device.Name, 65335, true, 3)
	if err != nil {
		return err
	}
	defer handle.Close()
	netIface := getNetInterface(device.Addresses[0].IP)
	eth := layers.Ethernet{
		SrcMAC:       netIface.HardwareAddr,
		DstMAC:       []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF},
		EthernetType: layers.EthernetTypeARP,
	}
	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     6,
		ProtAddressSize:   4,
		Operation:         layers.ARPRequest,
		SourceHwAddress:   []byte(netIface.HardwareAddr),
		SourceProtAddress: []byte(device.Addresses[0].IP),
		DstHwAddress:      []byte{0, 0, 0, 0, 0, 0},
	}
	ipNet, err := parseCIDR(cidr)
	if err != nil {
		return err
	}
	ips_range := ipsInRange(ipNet)
	go func() {
		var wg sync.WaitGroup
		wg.Add(len(ips_range))
		for _, ip := range ips_range {
			go func(ip net.IP) {
				defer wg.Done()
				select {
				case <-ctx.Done():
					return
				default:
					localARP := arp
					localARP.DstProtAddress = []byte(ip.To4())

					buffer := gopacket.NewSerializeBuffer()
					opts := gopacket.SerializeOptions{
						FixLengths:       true,
						ComputeChecksums: true,
					}
					err := gopacket.SerializeLayers(buffer, opts, &eth, &localARP)
					if err != nil {
						log.Println("Ошибка сериализации пакета", err)
					}
					handle.WritePacketData(buffer.Bytes())
				}
			}(ip)
		}
		wg.Wait()
	}()

	handle.SetBPFFilter("arp")
	timeout := time.After(30 * time.Second)
	flag := false
	for !flag {
		select {
		case <-timeout:
			flag = true
		case <-ctx.Done():
			flag = true
		default:
			packedData, _, err := handle.ReadPacketData()
			if err != nil {
				continue
			}
			packet := gopacket.NewPacket(packedData, layers.LayerTypeEthernet, gopacket.Default)
			if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
				arpPkt := arpLayer.(*layers.ARP)
				if arpPkt.Operation == layers.ARPReply {
					fmt.Printf("IP: %v\tMAC: %s\n", net.IP(arpPkt.SourceProtAddress), net.HardwareAddr(arpPkt.SourceHwAddress).String())
					ch <- fmt.Sprintf("IP: %v\tMAC: %s\n", net.IP(arpPkt.SourceProtAddress), net.HardwareAddr(arpPkt.SourceHwAddress).String())
				}
			}
		}
	}
	return nil
}

func getNetworkAdapters() ([]pcap.Interface, error) {
	devices, err := pcap.FindAllDevs()
	if err != nil {
		return []pcap.Interface{}, err
	}
	if len(devices) == 0 {
		err = fmt.Errorf("не найдено сетевых адаптеров")
	}
	return devices, err
}

func getListNetworkAdapters() []string {
	listDevices := []string{}
	devices, err := getNetworkAdapters()
	if err != nil {
		return []string{}
	}
	for i, d := range devices {
		listDevices = append(listDevices, fmt.Sprint(i+1)+" "+d.Name+" "+d.Description)
	}
	return listDevices
}

func getNetInterface(pcapInterfaceIP net.IP) net.Interface {
	var netIface net.Interface
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		addrs, _ := iface.Addrs()
		ip, _, _ := net.ParseCIDR(addrs[1].String())
		if ip.Equal(pcapInterfaceIP) {
			netIface = iface
		}
	}
	return netIface
}

func parseCIDR(cidr string) (*net.IPNet, error) {
	_, ipNet, err := net.ParseCIDR(cidr)
	return ipNet, err
}

func ipsInRange(ipNet *net.IPNet) []net.IP {
	ip_range := []net.IP{}
	ip := make(net.IP, len(ipNet.IP))
	copy(ip, ipNet.IP)
	for {
		new_ip := make(net.IP, len(ip))
		copy(new_ip, ip)
		ip_range = append(ip_range, new_ip)
		if !ipNet.Contains(ip) {
			break
		}
		nextIp(ip)
	}
	fmt.Println(len(ip_range))
	return ip_range
}
func nextIp(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		if (ip)[i] == 255 {
			(ip)[i] = 0
			continue
		}
		(ip)[i]++
		break
	}
}
