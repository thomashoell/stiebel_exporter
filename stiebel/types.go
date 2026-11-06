package stiebel

// ElsterType describes how the two value bytes of an Elster index are encoded.
type ElsterType int

// Value encodings of Elster indexes, see https://github.com/Hunv/can2mqtt and can_progs.
const (
	TypeNone          ElsterType = iota // signed 16 bit integer, big endian
	TypeDecVal                          // signed 16 bit integer / 10
	TypeTime                            // time of day, hh:mm
	TypeDate                            // day and month, dd.mm
	TypeDevID                           // device ID
	TypeByte                            // single byte
	TypeLittleEndian                    // signed 16 bit integer, little endian
	TypeDoubleVal                       // part of a value spread over two indexes
	TypeTripleVal                       // part of a value spread over three indexes
	TypeCentVal                         // signed 16 bit integer / 100
	TypeOperatingMode                   // operating mode, see OperatingModeName
	TypeBool                            // boolean, 0x0001 = true
	TypeMilVal                          // signed 16 bit integer / 1000
	TypeDevNr                           // device number
	TypeErrNr                           // error number, see ErrorText
	TypeTimeDomain                      // time range, hh:mm-hh:mm in 15 minute steps
	TypeLittleBool                      // boolean, 0x0100 = true
)

// ElsterReading is an Elster index (register) with its name and value encoding.
type ElsterReading struct {
	Name  string
	Index uint16
	Type  ElsterType
}

// Reading returns the definition of the given Elster index, or nil if the index is unknown.
func Reading(index uint16) *ElsterReading {
	return readingIndex[index]
}

// ErrorText returns the description of an error number reported by the heat pump.
func ErrorText(code uint16) (string, bool) {
	text, ok := errorCodes[code]
	return text, ok
}

// OperatingModeName returns the name of an operating mode value.
func OperatingModeName(mode uint16) (string, bool) {
	name, ok := operatingModes[mode]
	return name, ok
}
