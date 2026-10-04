package codesignalcli

type sigcallS044425946 struct {
	prefix    string
	v44425814 *sigbodyprojectConfigAuthoringPart5TestaBlankPrefixAnswerIsNe
}

func (sigRecv *sigcallS044425946) call() {

	for _, root := range sigRecv.v44425814.discovered.Roots {
		if sigRecv.prefix == root {
			sigRecv.v44425814.
				t.
				Fatalf("layer %q silently adopted discovered root %q as a prefix, Layers = %+v", sigRecv.v44425814.layer.Name, root, sigRecv.v44425814.result.Layers)
		}
	}
}
