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
	data := [...]byte{0x31, 0x00, 0xfa, 0xfe, 0x1e, 0x00, 0x00, 0x00}
	fmt.Printf("%x\n", data)
	module := data[0] >> 4
	action := data[0] & 0x0F
	target := int(data[1])<<8 + int(data[2])
	elsterIndex := int(data[3])<<8 + int(data[4])
	payload := int(data[5])<<8 + int(data[6])

	if target == 0x00fa {
		fmt.Println("Elster")
	}

	fmt.Printf("%x %x %x %x %x\n", module, action, target, elsterIndex, payload)
}
