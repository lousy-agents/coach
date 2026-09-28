package codesignalcli

type sigrecordsEqualS244641246 struct {
	a []nameStatusRecord
	b []nameStatusRecord
	i int
}

func (sigRecv *sigrecordsEqualS244641246) call() (bool, bool) {

	for j := range sigRecv.a[sigRecv.i].paths {
		if sigRecv.a[sigRecv.i].paths[j] != sigRecv.b[sigRecv.i].paths[j] {
			return false, true
		}
	}
	return false, false
}
