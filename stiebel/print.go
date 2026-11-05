package stiebel

import (
	"fmt"
	"strings"

	"github.com/brutella/can"
)

// FormatFrame formats a frame like candump from can-utils, followed by the decoded Elster index and,
// for responses with a known index, its name and value.
func FormatFrame(frm can.Frame) string {
	length := min(int(frm.Length), len(frm.Data))
	data := frm.Data[:length]

	var sb strings.Builder
	fmt.Fprintf(&sb, "%-4X [%d] % -24X %-10s", frm.ID, frm.Length, data, "'"+printableString(data)+"'")

	msg, err := ParseMessage(data)
	if err != nil {
		return sb.String()
	}
	fmt.Fprintf(&sb, " %4X %04X", msg.Receiver, msg.Index)

	if msg.Type != Response {
		return sb.String()
	}
	if r := Reading(msg.Index); r != nil {
		val, err := DecodeValue(msg.Value, r.Type)
		if err != nil {
			fmt.Fprintf(&sb, " %-32s %11s", left(r.Name, 32), err)
		} else {
			fmt.Fprintf(&sb, " %-32s %11s", left(r.Name, 32), ValueString(val))
		}
	}

	return sb.String()
}

// ValueString formats a value returned by DecodeValue.
func ValueString(val any) string {
	switch v := val.(type) {
	case float64:
		return fmt.Sprintf("%.1f", v)
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return "<nil>"
	default:
		return fmt.Sprintf("0x%04X", v)
	}
}

func left(s string, chars int) string {
	if chars < len(s) {
		return s[:chars]
	}
	return s
}

// printableString creates a string from s and replaces non-printable bytes with '.',
// similar to candump from can-utils.
func printableString(s []byte) string {
	ascii := make([]byte, len(s))
	for i, b := range s {
		if b < 32 || b > 126 {
			b = '.'
		}
		ascii[i] = b
	}
	return string(ascii)
}
