// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package chain

import (
	"fmt"
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/core/types"
	"github.com/BOXFoundation/boxd/storage"
	"github.com/BOXFoundation/boxd/util/bloom"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

var (
	db    = initDB()
	tests = [][]byte{
		[]byte("99108ad8ed9bb6274d3980bab5a85c048f0950c8"),
		[]byte("19108ad8ed9bb6274d3980bab5a85c048f0950c8"),
		[]byte("b5a2c786d9ef4658287ced5914b37a1b4aa32eee"),
		[]byte("b9300670b4c5366e95b2699e8b18bc75e5f729c5"),
		[]byte("a5ddc786d9ef4658287ced5914b37a1b456a2fff"),
		[]byte("a5ddc786d9ef4658892ced1234b37bd6456a2fff"),
	}
	topicslist = [][][]byte{
		[][]byte{
			[]byte("99108ad8ed9bb6274d3980bab5a85c048f0950c8"),
			[]byte("19108ad8ed9bb6274d3980bab5a85c048f0950c8"),
		},
		[][]byte{
			[]byte("b5a2c786d9ef4658287ced5914b37a1b4aa32eee"),
			[]byte("b9300670b4c5366e95b2699e8b18bc75e5f729c5"),
		},
		[][]byte{
			[]byte("a5ddc786d9ef4658287ced5914b37a1b456a2fff"),
			[]byte("a5ddc786d9ef4658892ced1234b37bd6456a2fff"),
		},
	}
)

func initDB() *storage.Database {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}
	proc := goprocess.WithSignals(os.Interrupt)
	database, _ := storage.NewDatabase(proc, dbCfg)
	return database
}

func TestIndex(t *testing.T) {
	// Init section manager.
	mgr, err := NewSectionManager(db)
	ensure.Nil(t, err)

	emptyBF := bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)
	testBF := bloom.NewFilterWithMK(types.BloomBitLength, types.BloomHashNum)
	for _, d := range tests {
		testBF.Add(d)
	}

	for i := 0; i < SectionBloomLength*3/2; i++ {
		if i%8 == 0 {
			err = mgr.AddBloom(uint(i%SectionBloomLength), testBF)
			ensure.Nil(t, err)
		} else {
			err = mgr.AddBloom(uint(i%SectionBloomLength), emptyBF)
		}
	}

	// Get all marked heights.
	heights, err := mgr.Index(2500, 10000, topicslist)
	ensure.Nil(t, err)
	fmt.Println(heights)
}
