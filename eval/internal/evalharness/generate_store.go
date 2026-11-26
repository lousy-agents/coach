package evalharness

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func storeFor(request GenerateRequest) (evalgen.Store, error) {
	if _, err := exec.LookPath("go"); err != nil {
		return evalgen.Store{}, fmt.Errorf("the go toolchain is not on PATH: generated programs are validated with go build, vet and test")
	}
	store := evalgen.DefaultStore()
	if request.Panel == PanelHeldOut {
		store.OracleRoot = os.Getenv("EVAL_ORACLE_DIR")
		if store.OracleRoot == "" {
			return store, fmt.Errorf("the held_out panel needs EVAL_ORACLE_DIR, the directory that keeps its oracle apart from the programs")
		}
	}
	return store, nil
}
