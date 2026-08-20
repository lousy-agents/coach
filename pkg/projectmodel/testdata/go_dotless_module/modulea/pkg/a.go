package pkg

import (
	"fmt"

	exclpkg "github.com/excluded/pkg"
	extpkg "github.com/external/pkg"
	replpkg "github.com/replaced/pkg"
	unresolvedpkg "github.com/unresolved/pkg"
	modbgreet "moduleab/greet"
)

// UseAll references every import kind exercised by the go_dotless_module
// fixture: internal (modbgreet, resolved through a dotless module path that
// would otherwise be misclassified as stdlib), stdlib (fmt), external
// (extpkg), replaced (replpkg), excluded (exclpkg), and unresolved
// (unresolvedpkg).
func UseAll() string {
	return fmt.Sprintf("%v %v %v %v %v", modbgreet.Hello, extpkg.Name, replpkg.Name, exclpkg.Name, unresolvedpkg.Name)
}
