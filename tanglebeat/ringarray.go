package main

type ringArray struct {
	curr int
	arr  []uint64
}

func newRingArray(len int) *ringArray {
	if len <= 0 {
		panic("len must be positive")
	}
	return &ringArray{
		arr: make([]uint64, len),
	}
}

func (ra *ringArray) get(idx int) uint64 {
	return ra.arr[idx%len(ra.arr)]
}

func (ra *ringArray) put(idx int, value uint64) {
	ra.arr[idx%len(ra.arr)] = value
}

func (ra *ringArray) push(value uint64) {
	ra.arr[ra.curr] = value
	ra.curr = (ra.curr + 1) % len(ra.arr)
}

func (ra *ringArray) takeAndNext() uint64 {
	ret := ra.arr[ra.curr]
	ra.curr = (ra.curr + 1) % len(ra.arr)
	return ret
}

func (ra *ringArray) len() int {
	return len(ra.arr)
}

func (ra *ringArray) sum() uint64 {
	var ret uint64
	for _, b := range ra.arr {
		ret += b
	}
	return ret
}

func (ra *ringArray) numzeros() uint64 {
	var ret uint64
	for _, b := range ra.arr {
		if b == 0 {
			ret += 1
		}
	}
	return ret
}
