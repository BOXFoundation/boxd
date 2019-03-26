package trie

import (
	"os"
	"testing"

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

	_k2, _ := util.FromHex("abcdfe")
	v2 := []byte("v2")
	ensure.Nil(t, trie.Update(_k2, v2))
	v, err = trie.Get(_k2)
	ensure.Nil(t, err)
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

}
