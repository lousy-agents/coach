package main

type scanOfferKind string

const scanOfferCompilerSetup scanOfferKind = "compiler_setup"

type scanOfferBudget map[scanOfferKind]bool

func newScanOfferBudget() scanOfferBudget {
	return scanOfferBudget{scanOfferCompilerSetup: true}
}

func (b scanOfferBudget) allows(kind scanOfferKind) bool {
	return b[kind]
}

func (b scanOfferBudget) without(kind scanOfferKind) scanOfferBudget {
	next := make(scanOfferBudget, len(b))
	for k, v := range b {
		next[k] = v
	}
	next[kind] = false
	return next
}
