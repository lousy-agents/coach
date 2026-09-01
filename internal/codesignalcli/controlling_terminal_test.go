package codesignalcli

import (
	"os"
	"testing"
)

func TestHasControllingTerminal(t *testing.T) {
	t.Run("regular file is not a controlling terminal", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "not-a-tty")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		defer f.Close()

		if got := HasControllingTerminal(f); got {
			t.Fatalf("HasControllingTerminal(regular file) = %v, want false", got)
		}
	})

	t.Run("pipe is not a controlling terminal", func(t *testing.T) {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("Pipe: %v", err)
		}
		defer r.Close()
		defer w.Close()

		if got := HasControllingTerminal(r); got {
			t.Fatalf("HasControllingTerminal(pipe read end) = %v, want false", got)
		}
	})

	t.Run("nil file is not a controlling terminal", func(t *testing.T) {
		if got := HasControllingTerminal(nil); got {
			t.Fatalf("HasControllingTerminal(nil) = %v, want false", got)
		}
	})

	t.Run("/dev/null is a character device but not a controlling terminal", func(t *testing.T) {
		f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
		if err != nil {
			t.Fatalf("OpenFile(%s): %v", os.DevNull, err)
		}
		defer f.Close()

		if got := HasControllingTerminal(f); got {
			t.Fatalf("HasControllingTerminal(%s) = %v, want false", os.DevNull, got)
		}
	})

	t.Run("pty slave is a controlling terminal", func(t *testing.T) {
		tty := openPTYSlave(t)
		defer tty.Close()

		if got := HasControllingTerminal(tty); !got {
			t.Fatalf("HasControllingTerminal(pty slave) = %v, want true", got)
		}
	})
}
