package evalharness

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// toolsFor finds the toolchain the language's programs are validated with, before anything is generated.
func toolsFor(language evalgen.Language) (evalgen.TypeScriptTools, error) {
	if language == evalgen.LanguageTypeScript {
		workingDir, _ := os.Getwd()
		return evalgen.ResolveTypeScriptTools(workingDir)
	}
	if _, err := exec.LookPath("go"); err != nil {
		return evalgen.TypeScriptTools{}, fmt.Errorf("the go toolchain is not on PATH: generated programs are validated with go build, vet and test")
	}
	return evalgen.TypeScriptTools{}, nil
}

func generateProgram(request GenerateRequest) (evalgen.Program, error) {
	if request.Language == evalgen.LanguageTypeScript {
		return evalgen.GenerateTypeScript(request.Cell, request.Seed)
	}
	return evalgen.Generate(request.Cell, request.Seed)
}

func validateProgram(program evalgen.Program, tools evalgen.TypeScriptTools) error {
	if program.Language == evalgen.LanguageTypeScript {
		return evalgen.ValidateTypeScript(program, tools)
	}
	return evalgen.Validate(program)
}
