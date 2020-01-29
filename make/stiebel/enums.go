package stiebel

const (
	// The higher 4 bits of the first byte of a telegram indicate the recipient (module). The value is calculated from
	// the CAN ID if the recipient. For example, to send a telegram to the 'Kessel' module, 0x180 must be divided by 0x80
	// (equals 0x3). Then, for a read request, 0x1 must be used for the lower 4 bytes. The result of all this would be
	// 0x31. A write request would be represented by 0x30.
	moduleDirekt        module = 0x0	// 000 (direct)
	moduleLargeTelegram module = 0x2
	moduleKessel        module = 0x3	// 180 (Kessel)
										// 280 (atez ?)
	moduleControl       module = 0x6	// 300, 301, ...
										// 400 (Fernraumfühler)
	moduleManager       module = 0x9	// 480 (Manager)
	moduleHeizmodul     module = 0xA	// 500
										// 580 (Buskoppler)
	moduleMischermodul  module = 0xC	// 600, 601, ...
	modulePc            module = 0xD	// 6A1, 6A2 (ComfortSoft)

	// The lower 4 bits of the first byte of a telegram indicate the request type. See comment above for instructions.
	typeWrite        telegramType = 0x0
	typeRead         telegramType = 0x1
	typeResponse     telegramType = 0x2
	typeAck          telegramType = 0x3
	typeWriteAck     telegramType = 0x4
	typeWriteRespond telegramType = 0x5
	typeSystem       telegramType = 0x6
	typeSystemRepond telegramType = 0x7

	// Unit the response payload must be treated as.
	unitDefault      elsterUnit = 0
	unitDecimal      elsterUnit = 1
	unitByte         elsterUnit = 2
	unitLittleEndian elsterUnit = 3
	unitTime         elsterUnit = 4
	unitDate         elsterUnit = 5

	//ElsterIndex 0x0112
	emergency          programMode = 0
	standby            programMode = 1
	automatic          programMode = 2
	daytimeOperation   programMode = 3
	nighttimeOperation programMode = 4
	warmWater          programMode = 5

	german  language = 0
	english language = 1
	french  language = 2
)
