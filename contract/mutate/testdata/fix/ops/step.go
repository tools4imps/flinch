package ops

func Step(n int) (total int) {
	for i := 0; i < n; i++ {
		total += i
	}
	n--
	total -= n
	return
}
