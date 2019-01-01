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
	if ra == nil {
		return
	}
	ra.arr[ra.curr] = value
	ra.curr = (ra.curr + 1) % len(ra.arr)
	ra.full = ra.full || ra.curr == 0
}

func (ra *RingArray) Len() int {
	if ra == nil {
		return 0
	}
	if ra.full {
		return len(ra.arr)
	} else {
		return ra.curr
	}
}

func (ra *RingArray) Min() uint64 {
	if ra == nil || (!ra.full && ra.curr == 0) {
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

func (ra *RingArray) CountGT(n uint64) uint64 {
	if ra == nil {
		return 0
	}
	var ret uint64

	for i := 0; i < ra.Len(); i++ {
		if ra.arr[i] >= n {
			ret++
		}
	}
	return ret
}

func (ra *RingArray) SumGT(n uint64) uint64 {
	if ra == nil {
		return 0
	}
	var ret uint64
	for i := 0; i < ra.Len(); i++ {
		if ra.arr[i] >= n {
			ret += ra.arr[i]
		}
	}
	return ret
}

func (ra *RingArray) AvgGT(n uint64) uint64 {
	if ra == nil {
		return 0
	}
	k := ra.CountGT(n)
	if k == 0 {
		return 0
	}
	return ra.SumGT(n) / k
}

func (ra *RingArray) Reset() {
	if ra == nil {
		return
	}
	ra.full = false
	ra.curr = 0
}
