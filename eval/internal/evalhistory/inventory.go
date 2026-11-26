package evalhistory

import "errors"

var errStub = errors.New("evalhistory: not implemented")

type Inventory struct{ Functions, Files []Unit }

func GoInventory(repoDir, since string) (Inventory, error) { return Inventory{}, errStub }

type Signal struct {
	RuleID, Path, Subject string
	StartRow, EndRow      int
}

type Level string

const (
	FunctionLevel Level = "function"
	FileLevel     Level = "file"
)

type Flagged struct {
	Level    Level
	Units    []int
	Unplaced int
}

func FlaggedUnits(inventory Inventory, signals []Signal, ruleID string) Flagged { return Flagged{} }

type Draw struct {
	Controls  []int
	Shortfall int
}

func DrawControls(pool []Unit, flagged []int, k int, seed int64) Draw { return Draw{} }

type Association struct {
	RiskRatio, Low, High *float64
	Reason               string
}

func RiskRatio(flaggedInvolved, flagged, controlInvolved, controls int) Association {
	return Association{}
}
