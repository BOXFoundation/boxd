// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

// ScoreSorter asdfa
type scoreSorter []peerScore

func (ss scoreSorter) Len() int {
	return len(ss)
}

// Swap safa
func (ss scoreSorter) Swap(i, j int) {
	ss[i], ss[j] = ss[j], ss[i]
}

func (ss scoreSorter) Less(i, j int) bool {
	return ss[i].score > ss[j].score
}
