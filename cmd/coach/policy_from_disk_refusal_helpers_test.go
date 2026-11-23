package main

import (
	. "github.com/onsi/gomega"
)

// expectDiskJudgedRefusal asserts the refusal of a policy the working tree cannot supply: exit 2, no report, and
// stderr naming project_config_invalid, the path and every wanted word (the reason, or the stable cause).
func expectDiskJudgedRefusal(stdout, stderr []byte, exitCode int, path string, words ...string) {
	ExpectWithOffset(1, exitCode).To(Equal(2), "stdout: %s\nstderr: %s", stdout, stderr)
	ExpectWithOffset(1, string(stdout)).To(BeEmpty(), "a refusal writes no report")
	ExpectWithOffset(1, string(stderr)).To(ContainSubstring("project_config_invalid"))
	ExpectWithOffset(1, string(stderr)).To(ContainSubstring(path))
	for _, word := range words {
		ExpectWithOffset(1, string(stderr)).To(ContainSubstring(word))
	}
}
