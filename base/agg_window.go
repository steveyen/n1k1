//  Copyright (c) 2019 Couchbase, Inc.
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the
//  License. You may obtain a copy of the License at
//  http://www.apache.org/licenses/LICENSE-2.0
//  Unless required by applicable law or agreed to in writing,
//  software distributed under the License is distributed on an "AS
//  IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
//  express or implied. See the License for the specific language
//  governing permissions and limitations under the License.

package base

import (
	"github.com/couchbase/rhmap/store"
)

// WTok maps a window related configuration string to an internal
// token number, which enables faster, numeric comparisons.
var WTok = map[string]int{}

var WTokRows, WTokRange, WTokGroups, WTokUnbounded, WTokNum int
var WTokCurrentRow, WTokNoOthers, WTokGroup, WTokTies int

func init() {
	for tokenNum, tokenStr := range []string{
		"rows", "range", "groups", "unbounded", "num",
		"no-others", "group", "ties"} {
		WTok[tokenStr] = tokenNum
	}

	WTokRows, WTokRange, WTokGroups, WTokUnbounded, WTokNum =
		WTok["rows"], WTok["range"], WTok["groups"], WTok["unbounded"], WTok["num"]

	WTokCurrentRow, WTokNoOthers, WTokGroup, WTokTies =
		WTok["current-row"], WTok["no-others"], WTok["group"], WTok["ties"]
}

// -------------------------------------------------------------------

// WindowFrame represents an immutable window frame config along with
// a mutable current window frame that's associated with a window
// partition.
type WindowFrame struct {
	Type int // Ex: "rows", "range", "groups".

	BegBoundary int   // Ex: "unbounded", "num".
	BegNum      int64 // Used when beg boundary is "num".

	EndBoundary int   // Ex: "unbounded", "num".
	EndNum      int64 // Used when end boundary is "num".

	Exclude int // Ex: "current-row", "no-others", "group", "ties".

	// --------------------------------------------------------

	// Partition is the current window partition.
	Partition *store.Heap

	// --------------------------------------------------------

	// WindowFrameCurr tracks the current window frame, which is
	// updated as the caller steps through the window partition.
	WindowFrameCurr
}

// -------------------------------------------------------------------

type WindowFrameCurr struct {
	// Pos is mutated as the 0-based current pos is updated.
	Pos int64

	// Include is mutated as the current pos is updated.
	// Include represents the positions included in the current
	// window frame before positions are excluded.
	Include WindowSpan

	// Excludes is mutated as the current pos is updated.
	// Excludes may be empty, or might have multiple spans when
	// the exclude config is "group" or "ties".
	Excludes []WindowSpan
}

// -------------------------------------------------------------------

// WindowSpan represents a continuous range of [Beg, End) of positions
// in the current window partition. Beg >= End means an empty span.
type WindowSpan struct {
	Beg, End int64
}

// -------------------------------------------------------------------

func (wf *WindowFrame) Init(cfg interface{}, partition *store.Heap) {
	// Default window frame cfg according to standard is...
	// RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW EXCLUDE NO OTHERS
	parts := cfg.([]interface{})

	wf.Type = WTok[parts[0].(string)]

	wf.BegBoundary, wf.BegNum = WTok[parts[1].(string)], int64(parts[2].(int))
	wf.EndBoundary, wf.EndNum = WTok[parts[3].(string)], int64(parts[4].(int))

	wf.Exclude = WTok[parts[5].(string)]

	wf.Partition = partition
}

// -------------------------------------------------------------------

// PartitionStart is invoked whenever a new window partition has been
// seen -- which means reseting the current window frame.
func (wf *WindowFrameCurr) PartitionStart() {
	wf.Pos = -1
	wf.Include = WindowSpan{}
	wf.Excludes = wf.Excludes[:0]
}

// -------------------------------------------------------------------

// CurrentUpdate is invoked whenever the current row is updated and
// stepped to the next row, so we update the current window frame.
func (wf *WindowFrame) CurrentUpdate(currentPos uint64) {
	wf.Pos = int64(currentPos)

	// Default to unbounded preceding.
	wf.Include.Beg = 0

	if wf.BegBoundary == WTokNum {
		if wf.Type != WTokRows {
			panic("unsupported")
		}

		// Handle cases of current-row and expr preceding|following.
		wf.Include.Beg = wf.Pos + wf.BegNum
		if wf.Include.Beg < 0 {
			wf.Include.Beg = 0
		}
	}

	// Default to unbounded following.
	n := int64(wf.Partition.Len())

	wf.Include.End = n

	if wf.EndBoundary == WTokNum {
		if wf.Type != WTokRows {
			panic("unsupported")
		}

		// Handle cases of current-row and expr preceding|following.
		wf.Include.End = wf.Pos + wf.EndNum + 1
		if wf.Include.End > n {
			wf.Include.End = n
		}

	}

	// Default to excluded rows of no-others.
	wf.Excludes = wf.Excludes[:0]

	if wf.Exclude != WTokNoOthers {
		if wf.Exclude == WTokCurrentRow {
			wf.Excludes = append(wf.Excludes, WindowSpan{wf.Pos, wf.Pos + 1})
		} else {
			panic("unsupported")
		}
	}
}

// -------------------------------------------------------------------

// StepVals is used for iterating through the current window frame and
// returns the next vals & position given the last seen position.
func (wf *WindowFrame) StepVals(next bool, iLast int64, valsPre Vals) (
	vals Vals, i int64, ok bool, err error) {
	if next {
		i, ok = wf.Next(iLast)
	} else {
		i, ok = wf.Prev(iLast)
	}
	if ok {
		buf, err := wf.Partition.Get(int(i))
		if err != nil {
			return nil, -1, false, err
		}

		vals = ValsDecode(buf, valsPre[:0])
	}

	return vals, i, ok, nil
}

// -------------------------------------------------------------------

// Next is used for iterating through the current window frame and
// returns the next position given the last seen position.
func (wf *WindowFrameCurr) Next(i int64) (int64, bool) {
	if i < wf.Include.Beg {
		i = wf.Include.Beg
	} else {
		i++
	}

	for _, exclude := range wf.Excludes {
		if i >= exclude.Beg && i < exclude.End {
			i = exclude.End
		}
	}

	if i >= wf.Include.End {
		return i, false
	}

	return i, true
}

// -------------------------------------------------------------------

// Prev is used for iterating in reverse through the current window
// frame and returns the prev position given the last seen position.
func (wf *WindowFrameCurr) Prev(i int64) (int64, bool) {
	if i >= wf.Include.End {
		i = wf.Include.End - 1
	} else {
		i--
	}

	for j := len(wf.Excludes) - 1; j >= 0; j-- {
		// Examine the Excludes in reverse in case they're adjacent.
		exclude := &wf.Excludes[j]
		if i >= exclude.Beg && i < exclude.End {
			i = exclude.Beg - 1
		}
	}

	if i < wf.Include.Beg {
		return i, false
	}

	return i, true
}

// -------------------------------------------------------------------

// Count returns the number of rows in the current frame.
func (wf *WindowFrameCurr) Count() int64 {
	s := wf.Include.End - wf.Include.Beg

	for _, exclude := range wf.Excludes {
		if Overlaps(exclude.Beg, exclude.End,
			wf.Include.Beg, wf.Include.End) {
			s = s - (Min(exclude.End, wf.Include.End) -
				Max(exclude.Beg, wf.Include.Beg))
		}
	}

	return s
}

// -------------------------------------------------------------------

// Overlaps returns true if the range [xBeg, xEnd) overlaps with the
// range [yBeg, yEnd).
func Overlaps(xBeg, xEnd, yBeg, yEnd int64) bool {
	if xEnd <= yBeg || yEnd <= xBeg {
		return false
	}
	return true
}

// -------------------------------------------------------------------

// Max returns the greater of a and b.
func Max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// Min returns the lesser of a and b.
func Min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}