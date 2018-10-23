// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package key

import (
	"path"
	"strings"
)

// A Key represents the unitque identity of an object.
// Key schema likes file name of a file system,
//     Key("/tx/0af020217fd26e0b6bf40912bca223b1dd806a21")
//     Key("/block/0af020217fd26e0b6bf40912bca223b1dd806a21")
//     Key("/ttl/peer/a9f827938c7de0")
// Inspired by https://github.com/ipfs/go-datastore
type Key struct {
	string
}

// NewKey constructs a key from string. it will clean the value.
func NewKey(s string) Key {
	k := Key{s}
	k.Clean()
	return k
}

// NewKeyFromBytes constructs a key from byte slice. it will clean the value.
func NewKeyFromBytes(s []byte) Key {
	return NewKey(string(s))
}

// RawKey creates a new Key without safety checking the input. Use with care.
func RawKey(s string) Key {
	// accept an empty string and fix it to avoid special cases
	// elsewhere
	if len(s) == 0 {
		return Key{"/"}
	}

	// perform a quick sanity check that the key is in the correct
	// format, if it is not then it is a programmer error and it is
	// okay to panic
	if len(s) == 0 || s[0] != '/' || (len(s) > 1 && s[len(s)-1] == '/') {
		panic("invalid datastore key: " + s)
	}

	return Key{s}
}

// NewKeyWithPaths constructs a key out of a path slice.
func NewKeyWithPaths(p ...string) Key {
	return NewKey(strings.Join(p, "/"))
}

// NewKeyWithPathList constructs a key out of a path slice.
func NewKeyWithPathList(l []string) Key {
	return NewKey(strings.Join(l, "/"))
}

// Clean up a Key, using path.Clean.
func (k *Key) Clean() {
	switch {
	case len(k.string) == 0:
		k.string = "/"
	case k.string[0] == '/':
		k.string = path.Clean(k.string)
	default:
		k.string = path.Clean("/" + k.string)
	}
}

// Strings is the string value of Key
func (k Key) String() string {
	return k.string
}

// Bytes returns the string value of Key as a []byte
func (k Key) Bytes() []byte {
	return []byte(k.string)
}

// Equal checks equality of two keys
func (k Key) Equal(k2 Key) bool {
	return k.string == k2.string
}

// Less checks whether this key is sorted lower than another.
func (k Key) Less(k2 Key) bool {
	list1 := k.List()
	list2 := k2.List()
	for i, c1 := range list1 {
		if len(list2) < (i + 1) {
			return false
		}

		c2 := list2[i]
		if c1 < c2 {
			return true
		} else if c1 > c2 {
			return false
		}
		// c1 == c2, continue
	}

	// list1 is shorter or exactly the same.
	return len(list1) < len(list2)
}

// List returns the `list` representation of this Key.
//   NewKey("/block/0af020217fd26e0b6bf40912bca223b1dd806a21").List()
//   ["block", "0af020217fd26e0b6bf40912bca223b1dd806a21"]
func (k Key) List() []string {
	return strings.Split(k.string, "/")[1:]
}

// BaseName returns the basename of this key like path.Base(filename)
func (k Key) BaseName() string {
	list := k.List()
	return list[len(list)-1]
}

// Base returns the base key of this key like path.Base(filename)
func (k Key) Base() Key {
	return NewKey(k.BaseName())
}

// Parent returns the `parent` Key of this Key.
func (k Key) Parent() Key {
	n := k.List()
	if len(n) == 1 {
		return RawKey("/")
	}
	return NewKey(strings.Join(n[:len(n)-1], "/"))
}

// Child returns the `child` Key of this Key.
func (k Key) Child(k2 Key) Key {
	switch {
	case k.string == "/":
		return k2
	case k2.string == "/":
		return k
	default:
		return RawKey(k.string + k2.string)
	}
}

// ChildString returns the `child` Key of this Key -- string helper.
func (k Key) ChildString(s string) Key {
	if len(s) == 0 {
		return k
	}
	if s[0] != '/' {
		s = "/" + s
	}
	return NewKey(k.string + s)
}

// IsAncestorOf returns whether this key is a prefix of `other`
//   NewKey("/Ancestor").IsAncestorOf("/Ancestor/Child")
//   true
func (k Key) IsAncestorOf(other Key) bool {
	if other.string == k.string {
		return false
	}
	return strings.HasPrefix(other.string, k.string)
}

// IsDescendantOf returns whether this key contains another as a prefix.
//   NewKey("/Ancestor/Child").IsDescendantOf("/Ancestor")
//   true
func (k Key) IsDescendantOf(other Key) bool {
	if other.string == k.string {
		return false
	}
	return strings.HasPrefix(k.string, other.string)
}

// IsTopLevel returns whether this key has only one namespace.
func (k Key) IsTopLevel() bool {
	return len(k.List()) == 1
}

// Slice attaches the methods of sort.Interface to []Key,
// sorting in increasing order.
type Slice []Key

func (p Slice) Len() int           { return len(p) }
func (p Slice) Less(i, j int) bool { return p[i].Less(p[j]) }
func (p Slice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
