package stiebel

import (
	"fmt"
	"testing"
)

func TestDecodeValue(t *testing.T) {
	b := []byte{0x80, 0xFF}

	r := DecodeValue(b, et_dec_val)

	if r != -25.5 {
		t.Fatalf(fmt.Sprintf("Expected %f, got %f", -25.5, r))
	}

	b = []byte{0x00, 0xFF}

	r = DecodeValue(b, et_dec_val)

	if r != 25.5 {
		t.Fatalf(fmt.Sprintf("Expected %f, got %f", 25.5, r))
	}


}


