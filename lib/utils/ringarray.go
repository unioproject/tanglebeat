package utils

type RingArray struct {
	full bool
	curr int
	arr  []uint64
}

func NewRingArray(len int) *RingArray {
	if len <= 0 {
		panic("Len must be positive")
	}
	return &RingArray{
		arr: make([]uint64, len),
	}
}

func (ra *RingArray) Push(value uint64) {
	ra.arr[ra.curr] = value
	ra.curr = (ra.curr + 1) % len(ra.arr)
	ra.full = ra.full || ra.curr == 0
}

func (ra *RingArray) TakeAndNext() uint64 {
	ret := ra.arr[ra.curr]
	ra.curr = (ra.curr + 1) % len(ra.arr)
	ra.full = ra.full || ra.curr == 0
	return ret
}

func (ra *RingArray) Len() int {
	if ra.full {
		return len(ra.arr)
	} else {
		return ra.curr
	}
}

func (ra *RingArray) Sum() uint64 {
	var ret uint64
	for i := 0; i < ra.Len(); i++ {
		ret += ra.arr[i]
	}
	return ret
}

func (ra *RingArray) Min() uint64 {
	if !ra.full && ra.curr == 0 {
		return 0
	}
	ret := ra.arr[0]
	for i := 0; i < ra.Len(); i++ {
		if ra.arr[i] < ret {
			ret = ra.arr[i]
		}
	}
	return ret
}

func (ra *RingArray) NumGT(n uint64) uint64 {
	var ret uint64

	for i := 0; i < ra.Len(); i++ {
		if ra.arr[i] >= n {
			ret++
		}
	}
	return ret
}

func (ra *RingArray) Reset() {
	ra.full = false
	ra.curr = 0
}
