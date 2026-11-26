package evalgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultStore is $XDG_CACHE_HOME/coach-eval/fixtures.
func DefaultStore() Store {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".cache")
	}
	return Store{Root: filepath.Join(base, "coach-eval", "fixtures")}
}

// Key hashes everything that decides the emitted bytes.
func Key(language Language, cell Cell, seed int64, panel string) string {
	cellJSON, _ := json.Marshal(cell)
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%d|%s|operators=", language, language.Version(), cellJSON, seed, panel)))
	return hex.EncodeToString(sum[:])
}
