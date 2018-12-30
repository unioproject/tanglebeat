package utils

type RingArray struct {
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

func (ra *RingArray) Get(idx int) uint64 {
	return ra.arr[idx%len(ra.arr)]
}

func (ra *RingArray) Put(idx int, value uint64) {
	ra.arr[idx%len(ra.arr)] = value
}

func (ra *RingArray) Push(value uint64) {
	ra.arr[ra.curr] = value
	ra.curr = (ra.curr + 1) % len(ra.arr)
}

func (ra *RingArray) TakeAndNext() uint64 {
	ret := ra.arr[ra.curr]
	ra.curr = (ra.curr + 1) % len(ra.arr)
	return ret
}

func (ra *RingArray) Len() int {
	return len(ra.arr)
}

func (ra *RingArray) Sum() uint64 {
	var ret uint64
	for _, b := range ra.arr {
		ret += b
	}
	return ret
}

func (ra *RingArray) Numzeros() uint64 {
	var ret uint64
	for _, b := range ra.arr {
		if b == 0 {
			ret += 1
		}
	}
	return ret
}
