package codesignalcli

func body_projectTsSetupExecuteKindTest_27(fakeKind string, original setupCommandTemplate, hadOriginal bool) {
	if hadOriginal {
		setupCommandTemplates[fakeKind] = original
	} else {
		delete(setupCommandTemplates, fakeKind)
	}
}
