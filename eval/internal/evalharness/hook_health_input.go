package evalharness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

func readHealthPrograms(stdin io.Reader) ([]healthProgram, error) {
	var programs []healthProgram
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var program healthProgram
		if err := json.Unmarshal([]byte(line), &program); err != nil {
			return nil, fmt.Errorf("%w: a line of standard input is not a program from `coach-eval generate`: %q", errHookHealthInput, line)
		}
		if info, err := os.Stat(program.Path); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w: the program %s has no directory at %s", errHookHealthInput, program.ProgramID, program.Path)
		}
		programs = append(programs, program)
	}
	if len(programs) == 0 {
		return nil, fmt.Errorf("%w: no program on standard input; pipe `coach-eval generate --tuning` into it", errHookHealthInput)
	}
	return programs, nil
}
