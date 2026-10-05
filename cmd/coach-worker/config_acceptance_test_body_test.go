package main

import (
	"os"
)

func body_configAcceptanceTest_23(envKeys []string) {
	for _, k := range envKeys {
		_ = os.Unsetenv(k)
	}
}
