package codesignalcli

import (
	"path/filepath"
	"runtime"
)

func npmArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "ia32"
	default:
		return runtime.GOARCH
	}
}

func nativeTypescriptUnscopedName() string {
	return "typescript-" + runtime.GOOS + "-" + npmArch()
}

func NativeTypescriptPackageName() string {
	return "@typescript/" + nativeTypescriptUnscopedName()
}

func nativePackageDirNextTo(compilerPackageDir string) string {
	return filepath.Join(filepath.Dir(compilerPackageDir), "@typescript", nativeTypescriptUnscopedName())
}

func resolveNativePackage(compilerPackageDir, compilerVersion string) (dir, foundVersion string, ok bool) {
	dir = nativePackageDirNextTo(compilerPackageDir)
	version, exists, unreadable := readTypescriptVersionAt(dir)
	if unreadable || !exists {
		return dir, "", false
	}
	if version != compilerVersion {
		return dir, version, false
	}
	return dir, version, true
}
