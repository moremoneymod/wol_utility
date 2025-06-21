package main

import (
	"fmt"
	"net"
)

func sendMagicPacket(mac string) error {
	hwAddr, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("неверный mac-адрес: %v", err)
	}
	magicPacket := createMagicPacket(hwAddr)
	addr := &net.UDPAddr{IP: net.IPv4bcast, Port: 9}
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return fmt.Errorf("ошибка подключения к UDP: %v", err)
	}
	defer conn.Close()
	res, err := conn.Write(magicPacket)
	fmt.Println(res)
	if err != nil {
		return fmt.Errorf("ошибка отправки пакета: %v", err)
	}
	return nil
}

func createMagicPacket(mac net.HardwareAddr) []byte {
	var packet [102]byte
	offset := 0
	for i := range 6 {
		packet[offset+i] = 0xFF
	}
	offset += 6
	for range 16 {
		copy(packet[offset:], mac)
		offset += 6
	}
	return packet[:]
}
