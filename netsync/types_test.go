package netsync

import (
	"testing"

	"github.com/BOXFoundation/boxd/crypto"
)

func TestLocateHeaders(t *testing.T) {
	locateHeaders := LocateHeaders{
		Headers: []*crypto.HashType{
			&crypto.HashType{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa,
				0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
				0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f},
			&crypto.HashType{0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa,
				0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16,
				0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x10, 0x0a},
		},
	}
	data, err := locateHeaders.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	//t.Log(data)
	//data = []byte{10, 32, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	//	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31}
	//data = []byte{10, 31, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	//	16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30}
	gotLocateHeaders := new(LocateHeaders)
	err = gotLocateHeaders.Unmarshal(data)
	if err != nil {
		t.Fatal(err)
	}
	if *gotLocateHeaders.Headers[0] != [crypto.HashSize]byte{
		0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
		16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31} ||
		*gotLocateHeaders.Headers[1] != [crypto.HashSize]byte{
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
			16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 16, 10} {
		t.Fatalf("want: %+v, got: %+v", locateHeaders, gotLocateHeaders)
	}
}
