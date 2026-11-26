package evalharness

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

func storeFor(request GenerateRequest) (evalgen.Store, error) {
	store := evalgen.DefaultStore()
	if request.Panel == PanelHeldOut {
		store.OracleRoot = os.Getenv("EVAL_ORACLE_DIR")
		if store.OracleRoot == "" {
			return store, fmt.Errorf("the held_out panel needs EVAL_ORACLE_DIR, the directory that keeps its oracle apart from the programs")
		}
	}
	return store, nil
}
