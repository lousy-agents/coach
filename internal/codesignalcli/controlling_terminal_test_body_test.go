package codesignalcli

import (
	"os"
	"testing"
)

func body_controllingTerminalTest_regularFileIsNotAControllingTerminal_9(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "not-a-tty")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()

	if got := HasControllingTerminal(f); got {
		t.Fatalf("HasControllingTerminal(regular file) = %v, want false", got)
	}
}

func body_controllingTerminalTest_pipeIsNotAControllingTerminal_21(t *testing.T) {
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

func body_controllingTerminalTest_nilFileIsNotAControllingTerminal_34(t *testing.T) {
	if got := HasControllingTerminal(nil); got {
		t.Fatalf("HasControllingTerminal(nil) = %v, want false", got)
	}
}

func body_controllingTerminalTest_devNullIsACharacterDeviceButNotAControllingTermi_40(t *testing.T) {
	f, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("OpenFile(%s): %v", os.DevNull, err)
	}
	defer f.Close()

	if got := HasControllingTerminal(f); got {
		t.Fatalf("HasControllingTerminal(%s) = %v, want false", os.DevNull, got)
	}
}

func body_controllingTerminalTest_ptySlaveIsAControllingTerminal_52(t *testing.T) {
	tty := openPTYSlave(t)
	defer tty.Close()

	if got := HasControllingTerminal(tty); !got {
		t.Fatalf("HasControllingTerminal(pty slave) = %v, want true", got)
	}
}
