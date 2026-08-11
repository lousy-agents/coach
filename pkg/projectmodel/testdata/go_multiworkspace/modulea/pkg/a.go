package pkg

import (
	"fmt"

	modbgreet "example.com/moduleb/greet"
	exclpkg "github.com/excluded/pkg"
	extpkg "github.com/external/pkg"
	replpkg "github.com/replaced/pkg"
	unresolvedpkg "github.com/unresolved/pkg"
)

// UseAll references every import kind exercised by the go_multiworkspace
// fixture: internal (modbgreet), stdlib (fmt), external (extpkg), replaced
// (replpkg), excluded (exclpkg), and unresolved (unresolvedpkg).
func UseAll() string {
	return fmt.Sprintf("%v %v %v %v %v", modbgreet.Hello, extpkg.Name, replpkg.Name, exclpkg.Name, unresolvedpkg.Name)
}
