// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
package trie

import (
	"os"
	"testing"

	"github.com/BOXFoundation/boxd/core"
	"github.com/BOXFoundation/boxd/crypto"
	"github.com/BOXFoundation/boxd/storage"
	_ "github.com/BOXFoundation/boxd/storage/memdb"
	"github.com/BOXFoundation/boxd/util"
	"github.com/facebookgo/ensure"
	"github.com/jbenet/goprocess"
)

var db = initDB()

func initDB() *storage.Database {
	dbCfg := &storage.Config{
		Name: "memdb",
		Path: "~/tmp",
	}
	proc := goprocess.WithSignals(os.Interrupt)
	database, _ := storage.NewDatabase(proc, dbCfg)
	return database
}
func TestUpdate(t *testing.T) {

	// ***********************************************************************
	// test insert & update
	ensure.NotNil(t, db)
	trie, err := New(nil, db)
	ensure.Nil(t, err)
	_k1, _ := util.FromHex("abc2de")
	k1 := []byte{0xa, 0xb, 0xc, 0x2, 0xd, 0xe}
	v1 := []byte("v1")
	ensure.Nil(t, trie.Update(_k1, v1))
	v, err := trie.Get(_k1)
	ensure.Nil(t, err)
	ensure.DeepEqual(t, v, v1)

	nodeBin, _ := db.Get(trie.rootHash[:])
	node1 := new(Node)
	ensure.Nil(t, node1.Unmarshal(nodeBin))
	ensure.DeepEqual(t, node1.Value[0], k1)
	ensure.DeepEqual(t, node1.Value[1], v1)
	ensure.DeepEqual(t, trie.rootHash[:], crypto.Sha3256(nodeBin))

	leaf1 := &Node{Value: [][]byte{k1, v1, termintor}}
	leafBin, _ := leaf1.Marshal()
	hash1 := crypto.Sha3256(leafBin)
	ensure.DeepEqual(t, trie.rootHash[:], hash1)

	trie1, err := New(trie.rootHash, db)
	nodeBin1, _ := db.Get(trie1.rootHash[:])
	node11 := new(Node)
	ensure.Nil(t, node11.Unmarshal(nodeBin1))
	ensure.DeepEqual(t, node11.Value[0], k1)
	ensure.DeepEqual(t, node11.Value[1], v1)

	//*****************************************
	//             abc
	//              |
	//            2   d
	//           /     \
	//         de       fe
	//******************************************
	_k2, _ := util.FromHex("abcdfe")
	v2 := []byte("v2")
	ensure.Nil(t, trie.Update(_k2, v2))
	v, _ = trie.Get(_k2)
	ensure.DeepEqual(t, v, v2)

	leaf1.Value[0] = []byte{0xd, 0xe}
	leafBin, _ = leaf1.Marshal()
	hash1 = crypto.Sha3256(leafBin)

	leaf2 := &Node{Value: [][]byte{[]byte{0xf, 0xe}, v2, termintor}}
	leafBin2, _ := leaf2.Marshal()
	hash2 := crypto.Sha3256(leafBin2)

	branch := newEmptyBranchNode()
	branch.Value[0x2] = hash1
	branch.Value[0xd] = hash2
	branchBin, _ := branch.Marshal()
	hashBranch := crypto.Sha3256(branchBin)

	extension := newExtensionNode([]byte{0xa, 0xb, 0xc}, hashBranch)
	extensionBin, _ := extension.Marshal()
	extensionHash := crypto.Sha3256(extensionBin)
	ensure.DeepEqual(t, trie.rootHash[:], extensionHash)

	//*****************************************
	//              a
	//              |
	//            5   b
	//           /     \
	//        c6fe      c
	//                  |
	//                2   d
	//               /     \
	//             de       fe
	//******************************************
	_k3, _ := util.FromHex("a5c6fe")
	_kinvalid, _ := util.FromHex("a5c6f")
	_kmiss, _ := util.FromHex("a5c6fc")
	v3 := []byte("v3")
	ensure.Nil(t, trie.Update(_k3, v3))
	v, _ = trie.Get(_k3)
	ensure.DeepEqual(t, v, v3)

	_, err = trie.Get(_kinvalid)
	ensure.DeepEqual(t, err, core.ErrInvalidKeyPath)
	_, err = trie.Get(_kmiss)
	ensure.DeepEqual(t, err, core.ErrNodeNotFound)

	extension.Value[0] = []byte{0xc}
	extensionBin, _ = extension.Marshal()
	extensionHash = crypto.Sha3256(extensionBin)

	leaf3 := &Node{Value: [][]byte{[]byte{0xc, 0x6, 0xf, 0xe}, v3, termintor}}
	leafBin3, _ := leaf3.Marshal()
	hash3 := crypto.Sha3256(leafBin3)

	branch = newEmptyBranchNode()
	branch.Value[0x5] = hash3
	branch.Value[0xb] = extensionHash[:]
	branchBin, _ = branch.Marshal()
	hashBranch = crypto.Sha3256(branchBin)

	root := newExtensionNode([]byte{0xa}, hashBranch)
	rootBin, _ := root.Marshal()
	rootHash := crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	//*****************************************
	//                         a
	//                         |
	//                   5     b     f
	//                  /      |      \
	//               c6fe      c      38ae
	//                         |
	//                       2   d
	//                      /     \
	//                    de       fe
	//******************************************
	_k4, _ := util.FromHex("af38ae")
	v4 := []byte("v4")
	ensure.Nil(t, trie.Update(_k4, v4))
	v, _ = trie.Get(_k4)
	ensure.DeepEqual(t, v, v4)

	leaf4 := &Node{Value: [][]byte{[]byte{0x3, 0x8, 0xa, 0xe}, v4, termintor}}
	leafBin4, _ := leaf4.Marshal()
	hash4 := crypto.Sha3256(leafBin4)

	branch.Value[0xf] = hash4
	branchBin, _ = branch.Marshal()
	hashBranch = crypto.Sha3256(branchBin)

	root.Value[1] = hashBranch
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	v5 := []byte("v5")
	ensure.Nil(t, trie.Update(_k4, v5))
	v, _ = trie.Get(_k4)
	ensure.DeepEqual(t, v, v5)
	ensure.Nil(t, trie.Update(_k4, v4))
	v, _ = trie.Get(_k4)
	ensure.DeepEqual(t, v, v4)

	// ***********************************************************************
	// delete v1
	//*****************************************
	//                         a
	//                         |
	//                   5     b     f
	//                  /      |      \
	//               c6fe     cdfe    38ae
	//*****************************************
	ensure.Nil(t, trie.Update(_k1, nil))
	_, err = trie.Get(_k1)
	ensure.DeepEqual(t, err, core.ErrNodeNotFound)
	v, _ = trie.Get(_k2)
	ensure.DeepEqual(t, v, v2)
	v, _ = trie.Get(_k3)
	ensure.DeepEqual(t, v, v3)
	v, _ = trie.Get(_k4)
	ensure.DeepEqual(t, v, v4)

	leafMiddle := &Node{Value: [][]byte{[]byte{0xc, 0xd, 0xf, 0xe}, v2, termintor}}
	leafMiddleBin, _ := leafMiddle.Marshal()
	hashMiddle := crypto.Sha3256(leafMiddleBin)

	branch.Value[0xb] = hashMiddle
	branchBin, _ = branch.Marshal()
	hashBranch = crypto.Sha3256(branchBin)

	root.Value[1] = hashBranch
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// delete v2
	//*****************************************
	//                       a
	//                       |
	//                   5       f
	//                  /         \
	//               c6fe         38ae
	//*****************************************
	ensure.Nil(t, trie.Update(_k2, nil))
	_, err = trie.Get(_k2)
	ensure.DeepEqual(t, err, core.ErrNodeNotFound)

	branch.Value[0xb] = nil
	branchBin, _ = branch.Marshal()
	hashBranch = crypto.Sha3256(branchBin)

	root.Value[1] = hashBranch
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// delete v3
	ensure.Nil(t, trie.Update(_k3, nil))
	_, err = trie.Get(_k3)
	ensure.DeepEqual(t, err, core.ErrNodeNotFound)

	root = &Node{Value: [][]byte{[]byte{0xa, 0xf, 0x3, 0x8, 0xa, 0xe}, v4, termintor}}
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// delete v4
	ensure.Nil(t, trie.Update(_k4, nil))
	_, err = trie.Get(_k4)
	ensure.DeepEqual(t, err, core.ErrNodeNotFound)
	ensure.DeepEqual(t, trie.rootHash, (*crypto.HashType)(nil))

	// ***********************************************************************

	_k1, _ = util.FromHex("c3af2f")
	k1 = []byte{0xc, 0x3, 0xa, 0xf, 0x2, 0xf}
	v1 = []byte("v1")
	ensure.Nil(t, trie.Update(_k1, v1))

	_k2, _ = util.FromHex("c3aa37")
	// k2 = []byte{0xc, 0x3, 0xa, 0xa, 0x3, 0x7}
	v2 = []byte("v2")
	ensure.Nil(t, trie.Update(_k2, v2))

	_k3, _ = util.FromHex("abcdef")
	// k3 = []byte{0xa, 0xb, 0xc, 0xd, 0xe, 0xf}
	v3 = []byte("v3")
	ensure.Nil(t, trie.Update(_k3, v3))

	_k4, _ = util.FromHex("c85f3a")
	// k3 = []byte{0xc, 0x8, 0x5, 0xf, 0x3, 0xa}
	v4 = []byte("v4")
	ensure.Nil(t, trie.Update(_k4, v4))

	//*****************************************
	//                       a    c
	//                      /     |
	//                 bcdef   3     8
	//                         |      \
	//                         a      5f3a
	//                         |
	//                       a   f
	//                      /     \
	//                    37       2f
	//******************************************
	leaf1 = &Node{Value: [][]byte{[]byte{0x2, 0xf}, v1, termintor}}
	leafBin, _ = leaf1.Marshal()
	hash1 = crypto.Sha3256(leafBin)

	leaf2 = &Node{Value: [][]byte{[]byte{0x3, 0x7}, v2, termintor}}
	leafBin2, _ = leaf2.Marshal()
	hash2 = crypto.Sha3256(leafBin2)

	branch1 := newEmptyBranchNode()
	branch1.Value[0xa] = hash2
	branch1.Value[0xf] = hash1
	branchBin1, _ := branch1.Marshal()
	hashBranch1 := crypto.Sha3256(branchBin1)

	extension1 := newExtensionNode([]byte{0xa}, hashBranch1)
	extensionBin1, _ := extension1.Marshal()
	extensionHash1 := crypto.Sha3256(extensionBin1)

	leaf4 = &Node{Value: [][]byte{[]byte{0x5, 0xf, 0x3, 0xa}, v4, termintor}}
	leafBin4, _ = leaf4.Marshal()
	hash4 = crypto.Sha3256(leafBin4)

	branch2 := newEmptyBranchNode()
	branch2.Value[0x3] = extensionHash1
	branch2.Value[0x8] = hash4
	branchBin2, _ := branch2.Marshal()
	hashBranch2 := crypto.Sha3256(branchBin2)

	leaf3 = &Node{Value: [][]byte{[]byte{0xb, 0xc, 0xd, 0xe, 0xf}, v3, termintor}}
	leafBin3, _ = leaf3.Marshal()
	hash3 = crypto.Sha3256(leafBin3)

	root = newEmptyBranchNode()
	root.Value[0xa] = hash3
	root.Value[0xc] = hashBranch2
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// delete _k1
	//*****************************************
	//                       a    c
	//                      /     |
	//                 bcdef   3     8
	//                        /       \
	//                      aa37      5f3a
	//******************************************
	ensure.Nil(t, trie.Update(_k1, nil))
	leaf2 = &Node{Value: [][]byte{[]byte{0xa, 0xa, 0x3, 0x7}, v2, termintor}}
	leafBin2, _ = leaf2.Marshal()
	hash2 = crypto.Sha3256(leafBin2)

	branch2.Value[0x3] = hash2
	branch2.Value[0x8] = hash4
	branchBin2, _ = branch2.Marshal()
	hashBranch2 = crypto.Sha3256(branchBin2)

	root.Value[0xa] = hash3
	root.Value[0xc] = hashBranch2
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// recover the trie
	ensure.Nil(t, trie.Update(_k1, v1))

	// delete _k2
	//*****************************************
	//                       a    c
	//                      /     |
	//                 bcdef   3     8
	//                        /       \
	//                      af2f      5f3a
	//******************************************
	ensure.Nil(t, trie.Update(_k2, nil))
	leaf1 = &Node{Value: [][]byte{[]byte{0xa, 0xf, 0x2, 0xf}, v1, termintor}}
	leafBin, _ = leaf1.Marshal()
	hash1 = crypto.Sha3256(leafBin)

	branch2.Value[0x3] = hash1
	branchBin2, _ = branch2.Marshal()
	hashBranch2 = crypto.Sha3256(branchBin2)

	root.Value[0xc] = hashBranch2
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// recover the trie
	ensure.Nil(t, trie.Update(_k2, v2))

	// delete _k3
	//*****************************************
	//                            c
	//                            |
	//                         3     8
	//                         |      \
	//                         a      5f3a
	//                         |
	//                       a   f
	//                      /     \
	//                    37       2f
	//******************************************
	ensure.Nil(t, trie.Update(_k3, nil))
	branch2.Value[0x3] = extensionHash1
	branch2.Value[0x8] = hash4
	branchBin2, _ = branch2.Marshal()
	hashBranch2 = crypto.Sha3256(branchBin2)

	root = newExtensionNode([]byte{0xc}, hashBranch2)
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)

	// recover the trie
	ensure.Nil(t, trie.Update(_k3, v3))

	// delete _k4
	//*****************************************
	//                       a    c
	//                      /     |
	//                 bcdef      3a
	//                            |
	//                          a   f
	//                         /     \
	//                       37       2f
	//******************************************
	ensure.Nil(t, trie.Update(_k4, nil))
	extension1 = newExtensionNode([]byte{0x3, 0xa}, hashBranch1)
	extensionBin1, _ = extension1.Marshal()
	extensionHash1 = crypto.Sha3256(extensionBin1)

	root = newEmptyBranchNode()
	root.Value[0xa] = hash3
	root.Value[0xc] = extensionHash1
	rootBin, _ = root.Marshal()
	rootHash = crypto.Sha3256(rootBin)
	ensure.DeepEqual(t, trie.rootHash[:], rootHash)
}
