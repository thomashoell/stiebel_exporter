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


