package rubrics_test

type sigbodypackAcceptanceTestemitsAStablePackCountAndNeverMerges struct {
	hasHot   *bool
	hasOther *bool
	hotPath  string
	paths    []string
}

func (sigRecv *sigbodypackAcceptanceTestemitsAStablePackCountAndNeverMerges) call() {

	for _, path := range sigRecv.paths {
		if path == sigRecv.hotPath {
			*sigRecv.hasHot = true
		} else {
			*sigRecv.hasOther = true
		}
	}
}
