package codesignalcli

import (
	"os"
	"testing"
)

func TestHasControllingTerminal_RegularFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "not-a-tty")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()

	if got := HasControllingTerminal(f); got {
		t.Fatalf("HasControllingTerminal(regular file) = %v, want false", got)
	}
}

func TestHasControllingTerminal_Pipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()

	if got := HasControllingTerminal(r); got {
		t.Fatalf("HasControllingTerminal(pipe read end) = %v, want false", got)
	}
}

func TestHasControllingTerminal_Nil(t *testing.T) {
	if got := HasControllingTerminal(nil); got {
		t.Fatalf("HasControllingTerminal(nil) = %v, want false", got)
	}
}

// TestHasControllingTerminal_DevNull is the AC-POL-9 case: /dev/null is a
// character device (os.ModeCharDevice is set), so a naive
// os.ModeCharDevice-based implementation wrongly reports it as a
// controlling terminal. A correct implementation must probe for an actual
// tty and return false here.
func TestHasControllingTerminal_DevNull(t *testing.T) {
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFile(%s): %v", os.DevNull, err)
	}
	defer f.Close()

	if got := HasControllingTerminal(f); got {
		t.Fatalf("HasControllingTerminal(%s) = %v, want false", os.DevNull, got)
	}
}

// TestHasControllingTerminal_PTY is the discriminating true case: without
// it, every case in this file expects false, so a stub that always
// returns false would pass the whole suite. A real pseudo-terminal slave
// is a genuine controlling terminal and must report true.
func TestHasControllingTerminal_PTY(t *testing.T) {
	tty := openPTYSlave(t)
	defer tty.Close()

	if got := HasControllingTerminal(tty); !got {
		t.Fatalf("HasControllingTerminal(pty slave) = %v, want true", got)
	}
}
