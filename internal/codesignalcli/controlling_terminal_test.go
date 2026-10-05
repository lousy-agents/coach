package codesignalcli

import (
	"testing"
)

func TestHasControllingTerminal(t *testing.T) {
	t.Run("regular file is not a controlling terminal", func(t *testing.T) {
		body_controllingTerminalTest_regularFileIsNotAControllingTerminal_9(t)
	})

	t.Run("pipe is not a controlling terminal", func(t *testing.T) {
		body_controllingTerminalTest_pipeIsNotAControllingTerminal_21(t)
	})

	t.Run("nil file is not a controlling terminal", func(t *testing.T) {
		body_controllingTerminalTest_nilFileIsNotAControllingTerminal_34(t)
	})

	t.Run("/dev/null is a character device but not a controlling terminal", func(t *testing.T) {
		body_controllingTerminalTest_devNullIsACharacterDeviceButNotAControllingTermi_40(t)
	})

	t.Run("pty slave is a controlling terminal", func(t *testing.T) {
		body_controllingTerminalTest_ptySlaveIsAControllingTerminal_52(t)
	})
}
