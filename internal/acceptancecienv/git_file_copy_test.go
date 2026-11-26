package acceptancecienv

import (
	"io"
	"os"
)

// copyGitFile copies one regular file, keeping its permission bits.
func copyGitFile(path, destination string, mode os.FileMode) error {
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
