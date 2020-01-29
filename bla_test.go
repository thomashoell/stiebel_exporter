package main

import (
	"fmt"
	"testing"
)

func Test(t *testing.T) {
	val := 0x180 >> 7
	fmt.Printf("%x\n", val)
	val = 0x31 - (0x3 << 4)
	fmt.Printf("%x\n", val)
}

func Test2(t *testing.T) {
	val := 0x31
	module := val >> 4
	action := val & 0x0F

	fmt.Printf("module: %x, action: %x", module, action)
}

func Test3(t *testing.T) {
	fmt.Printf("%x",0x11 << 8 + 0xfa)
}

func Test4(t *testing.T) {
	data := [...]byte{0xd2, 0x00, 0xfa, 0x01, 0xd6, 0x02, 0x0d, 0x00}
	fmt.Printf("%x\n", data)
	module := data[0] >> 4
	action := data[0] & 0x0F
	target := uint16(data[1])<<8 + uint16(data[2])
	elsterIndex := uint16(data[3])<<8 + uint16(data[4])
	payload := uint16(data[5])<<8 + uint16(data[6])

	if target == 0x00fa {
		fmt.Println("Elster")
	}

	fmt.Printf("%02x %02x %04x %04x %04x\n", module, action, target, elsterIndex, payload)
}
