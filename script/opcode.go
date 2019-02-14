// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package script

// OpCode enum
type OpCode byte

// These constants are based on bitcoin official opcodes
const (
	// push value
	OP0         OpCode = 0x00 // 0
	OPFALSE     OpCode = 0x00 // 0 - AKA OP0
	OPDATA20    OpCode = 0x14 // 20
	OPPUSHDATA1 OpCode = 0x4c // 76
	OPPUSHDATA2 OpCode = 0x4d // 77
	OPPUSHDATA4 OpCode = 0x4e // 78
	OP1NEGATE   OpCode = 0x4f // 79
	OPRESERVED  OpCode = 0x50 // 80
	OP1         OpCode = 0x51 // 81
	OPTRUE      OpCode = 0x51 // 81 - AKA OP1
	OP2         OpCode = 0x52 // 82
	OP3         OpCode = 0x53 // 83
	OP4         OpCode = 0x54 // 84
	OP5         OpCode = 0x55 // 85
	OP6         OpCode = 0x56 // 86
	OP7         OpCode = 0x57 // 87
	OP8         OpCode = 0x58 // 88
	OP9         OpCode = 0x59 // 89
	OP10        OpCode = 0x5a // 90
	OP11        OpCode = 0x5b // 91
	OP12        OpCode = 0x5c // 92
	OP13        OpCode = 0x5d // 93
	OP14        OpCode = 0x5e // 94
	OP15        OpCode = 0x5f // 95
	OP16        OpCode = 0x60 // 96

	// control
	OPNOP      OpCode = 0x61 // 97
	OPVER      OpCode = 0x62 // 98
	OPIF       OpCode = 0x63 // 99
	OPNOTIF    OpCode = 0x64 // 100
	OPVERIF    OpCode = 0x65 // 101
	OPVERNOTIF OpCode = 0x66 // 102
	OPELSE     OpCode = 0x67 // 103
	OPENDIF    OpCode = 0x68 // 104
	OPVERIFY   OpCode = 0x69 // 105
	OPRETURN   OpCode = 0x6a // 106

	// stack ops
	OPTOALTSTACK   OpCode = 0x6b // 107
	OPFROMALTSTACK OpCode = 0x6c // 108
	OP2DROP        OpCode = 0x6d // 109
	OP2DUP         OpCode = 0x6e // 110
	OP3DUP         OpCode = 0x6f // 111
	OP2OVER        OpCode = 0x70 // 112
	OP2ROT         OpCode = 0x71 // 113
	OP2SWAP        OpCode = 0x72 // 114
	OPIFDUP        OpCode = 0x73 // 115
	OPDEPTH        OpCode = 0x74 // 116
	OPDROP         OpCode = 0x75 // 117
	OPDUP          OpCode = 0x76 // 118
	OPNIP          OpCode = 0x77 // 119
	OPOVER         OpCode = 0x78 // 120
	OPPICK         OpCode = 0x79 // 121
	OPROLL         OpCode = 0x7a // 122
	OPROT          OpCode = 0x7b // 123
	OPSWAP         OpCode = 0x7c // 124
	OPTUCK         OpCode = 0x7d // 125

	// splice ops
	OPCAT    OpCode = 0x7e // 126
	OPSUBSTR OpCode = 0x7f // 127
	OPLEFT   OpCode = 0x80 // 128
	OPRIGHT  OpCode = 0x81 // 129
	OPSIZE   OpCode = 0x82 // 130

	// bit logic
	OPINVERT      OpCode = 0x83 // 131
	OPAND         OpCode = 0x84 // 132
	OPOR          OpCode = 0x85 // 133
	OPXOR         OpCode = 0x86 // 134
	OPEQUAL       OpCode = 0x87 // 135
	OPEQUALVERIFY OpCode = 0x88 // 136
	OPRESERVED1   OpCode = 0x89 // 137
	OPRESERVED2   OpCode = 0x8a // 138

	// numeric
	OP1ADD      OpCode = 0x8b // 139
	OP1SUB      OpCode = 0x8c // 140
	OP2MUL      OpCode = 0x8d // 141
	OP2DIV      OpCode = 0x8e // 142
	OPNEGATE    OpCode = 0x8f // 143
	OPABS       OpCode = 0x90 // 144
	OPNOT       OpCode = 0x91 // 145
	OP0NOTEQUAL OpCode = 0x92 // 146

	OPADD    OpCode = 0x93 // 147
	OPSUB    OpCode = 0x94 // 148
	OPMUL    OpCode = 0x95 // 149
	OPDIV    OpCode = 0x96 // 150
	OPMOD    OpCode = 0x97 // 151
	OPLSHIFT OpCode = 0x98 // 152
	OPRSHIFT OpCode = 0x99 // 153

	OPBOOLAND            OpCode = 0x9a // 154
	OPBOOLOR             OpCode = 0x9b // 155
	OPNUMEQUAL           OpCode = 0x9c // 156
	OPNUMEQUALVERIFY     OpCode = 0x9d // 157
	OPNUMNOTEQUAL        OpCode = 0x9e // 158
	OPLESSTHAN           OpCode = 0x9f // 159
	OPGREATERTHAN        OpCode = 0xa0 // 160
	OPLESSTHANOREQUAL    OpCode = 0xa1 // 161
	OPGREATERTHANOREQUAL OpCode = 0xa2 // 162
	OPMIN                OpCode = 0xa3 // 163
	OPMAX                OpCode = 0xa4 // 164

	OPWITHIN OpCode = 0xa5 // 165

	// crypto
	OPRIPEMD160           OpCode = 0xa6 // 166
	OPSHA1                OpCode = 0xa7 // 167
	OPSHA256              OpCode = 0xa8 // 168
	OPHASH160             OpCode = 0xa9 // 169
	OPHASH256             OpCode = 0xaa // 170
	OPCODESEPARATOR       OpCode = 0xab // 171
	OPCHECKSIG            OpCode = 0xac // 172
	OPCHECKSIGVERIFY      OpCode = 0xad // 173
	OPCHECKMULTISIG       OpCode = 0xae // 174
	OPCHECKMULTISIGVERIFY OpCode = 0xaf // 175
	OPCHECKLOCKTIMEVERIFY OpCode = 0xb0 // 176
)

// opCodeToName maps op code to name
func opCodeToName(opCode OpCode) string {
	switch opCode {
	// push value
	case OP0:
		return "OP_0"
	case OPPUSHDATA1:
		return "OP_PUSHDATA1"
	case OPPUSHDATA2:
		return "OP_PUSHDATA2"
	case OPPUSHDATA4:
		return "OP_PUSHDATA4"
	case OP1NEGATE:
		return "-1"
	case OPRESERVED:
		return "OP_RESERVED"
	case OP1:
		return "OP_1"
	case OP2:
		return "OP_2"
	case OP3:
		return "OP_3"
	case OP4:
		return "OP_4"
	case OP5:
		return "OP_5"
	case OP6:
		return "OP_6"
	case OP7:
		return "OP_7"
	case OP8:
		return "OP_8"
	case OP9:
		return "OP_9"
	case OP10:
		return "OP_10"
	case OP11:
		return "OP_11"
	case OP12:
		return "OP_12"
	case OP13:
		return "OP_13"
	case OP14:
		return "OP_14"
	case OP15:
		return "OP_15"
	case OP16:
		return "OP_16"

		// control
	case OPNOP:
		return "OP_NOP"
	case OPVER:
		return "OP_VER"
	case OPIF:
		return "OP_IF"
	case OPNOTIF:
		return "OP_NOTIF"
	case OPVERIF:
		return "OP_VERIF"
	case OPVERNOTIF:
		return "OP_VERNOTIF"
	case OPELSE:
		return "OP_ELSE"
	case OPENDIF:
		return "OP_ENDIF"
	case OPVERIFY:
		return "OP_VERIFY"
	case OPRETURN:
		return "OP_RETURN"

		// stack ops
	case OPTOALTSTACK:
		return "OP_TOALTSTACK"
	case OPFROMALTSTACK:
		return "OP_FROMALTSTACK"
	case OP2DROP:
		return "OP_2DROP"
	case OP2DUP:
		return "OP_2DUP"
	case OP3DUP:
		return "OP_3DUP"
	case OP2OVER:
		return "OP_2OVER"
	case OP2ROT:
		return "OP_2ROT"
	case OP2SWAP:
		return "OP_2SWAP"
	case OPIFDUP:
		return "OP_IFDUP"
	case OPDEPTH:
		return "OP_DEPTH"
	case OPDROP:
		return "OP_DROP"
	case OPDUP:
		return "OP_DUP"
	case OPNIP:
		return "OP_NIP"
	case OPOVER:
		return "OP_OVER"
	case OPPICK:
		return "OP_PICK"
	case OPROLL:
		return "OP_ROLL"
	case OPROT:
		return "OP_ROT"
	case OPSWAP:
		return "OP_SWAP"
	case OPTUCK:
		return "OP_TUCK"

		// splice ops
	case OPCAT:
		return "OP_CAT"
	case OPSUBSTR:
		return "OP_SUBSTR"
	case OPLEFT:
		return "OP_LEFT"
	case OPRIGHT:
		return "OP_RIGHT"
	case OPSIZE:
		return "OP_SIZE"

		// bit logic
	case OPINVERT:
		return "OP_INVERT"
	case OPAND:
		return "OP_AND"
	case OPOR:
		return "OP_OR"
	case OPXOR:
		return "OP_XOR"
	case OPEQUAL:
		return "OP_EQUAL"
	case OPEQUALVERIFY:
		return "OP_EQUALVERIFY"
	case OPRESERVED1:
		return "OP_RESERVED1"
	case OPRESERVED2:
		return "OP_RESERVED2"

		// numeric
	case OP1ADD:
		return "OP_1ADD"
	case OP1SUB:
		return "OP_1SUB"
	case OP2MUL:
		return "OP_2MUL"
	case OP2DIV:
		return "OP_2DIV"
	case OPNEGATE:
		return "OP_NEGATE"
	case OPABS:
		return "OP_ABS"
	case OPNOT:
		return "OP_NOT"
	case OP0NOTEQUAL:
		return "OP_0NOTEQUAL"
	case OPADD:
		return "OP_ADD"
	case OPSUB:
		return "OP_SUB"
	case OPMUL:
		return "OP_MUL"
	case OPDIV:
		return "OP_DIV"
	case OPMOD:
		return "OP_MOD"
	case OPLSHIFT:
		return "OP_LSHIFT"
	case OPRSHIFT:
		return "OP_RSHIFT"
	case OPBOOLAND:
		return "OP_BOOLAND"
	case OPBOOLOR:
		return "OP_BOOLOR"
	case OPNUMEQUAL:
		return "OP_NUMEQUAL"
	case OPNUMEQUALVERIFY:
		return "OP_NUMEQUALVERIFY"
	case OPNUMNOTEQUAL:
		return "OP_NUMNOTEQUAL"
	case OPLESSTHAN:
		return "OP_LESSTHAN"
	case OPGREATERTHAN:
		return "OP_GREATERTHAN"
	case OPLESSTHANOREQUAL:
		return "OP_LESSTHANOREQUAL"
	case OPGREATERTHANOREQUAL:
		return "OP_GREATERTHANOREQUAL"
	case OPMIN:
		return "OP_MIN"
	case OPMAX:
		return "OP_MAX"
	case OPWITHIN:
		return "OP_WITHIN"

		// crypto
	case OPRIPEMD160:
		return "OP_RIPEMD160"
	case OPSHA1:
		return "OP_SHA1"
	case OPSHA256:
		return "OP_SHA256"
	case OPHASH160:
		return "OP_HASH160"
	case OPHASH256:
		return "OP_HASH256"
	case OPCODESEPARATOR:
		return "OP_CODESEPARATOR"
	case OPCHECKSIG:
		return "OP_CHECKSIG"
	case OPCHECKSIGVERIFY:
		return "OP_CHECKSIGVERIFY"
	case OPCHECKMULTISIG:
		return "OP_CHECKMULTISIG"
	case OPCHECKMULTISIGVERIFY:
		return "OP_CHECKMULTISIGVERIFY"
	case OPCHECKLOCKTIMEVERIFY:
		return "OP_CHECKLOCKTIMEVERIFY"

	default:
		return "OP_UNKNOWN"
	}
}
