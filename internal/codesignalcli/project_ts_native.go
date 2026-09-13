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

// resolveNativePackage locates the platform-native
// @typescript/typescript-<os>-<arch> package for the compiler installed at
// compilerPackageDir. mise's default npm backend hoists the compiler
// itself to a symlink but not its native optionalDependency sibling
// (verified empirically against mise 2026.9.5), so that sibling is only
// ever found beside the real, symlink-resolved directory in that layout.
func resolveNativePackage(compilerPackageDir, compilerVersion string) (dir, foundVersion string, ok bool) {
	dir = nativePackageDirNextTo(compilerPackageDir)
	if version, found, matched := versionAt(dir, compilerVersion); found {
		return dir, version, matched
	}

	resolved, err := filepath.EvalSymlinks(compilerPackageDir)
	if err != nil || resolved == compilerPackageDir {
		return dir, "", false
	}
	realDir := nativePackageDirNextTo(resolved)
	version, found, matched := versionAt(realDir, compilerVersion)
	if !found {
		return dir, "", false
	}
	return realDir, version, matched
}

// versionAt reads the TypeScript version installed at dir. found is false
// when nothing readable is there; matched reports whether that version
// equals wantVersion.
func versionAt(dir, wantVersion string) (version string, found, matched bool) {
	version, exists, unreadable := readTypescriptVersionAt(dir)
	if !exists || unreadable {
		return "", false, false
	}
	return version, true, version == wantVersion
}
