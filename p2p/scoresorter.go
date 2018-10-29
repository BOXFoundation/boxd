// Copyright (c) 2018 ContentBox Authors.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package p2p

// ScoreSorter implements sort.Interface to allow a slice of peerConnScores to
// be sorted
type scoreSorter []peerConnScore

// Len returns the number of peerConnScores in the slice.  It is part of the
// sort.Interface implementation.
func (ss scoreSorter) Len() int {
	return len(ss)
}

// Swap swaps the peerConnScores at the passed indices.  It is part of the
// sort.Interface implementation.
func (ss scoreSorter) Swap(i, j int) {
	ss[i], ss[j] = ss[j], ss[i]
}

// Less returns whether the peerConnScore with index i should sort before the
// peerConnScore with index j.  It is part of the sort.Interface implementation.
func (ss scoreSorter) Less(i, j int) bool {
	return ss[i].score > ss[j].score
}
