package tstoolchain

import (
	"strconv"
)

var SupportedNodeMajors = []int{24, 26}

func NodeMajorSupported(major int) bool {
	for _, supported := range SupportedNodeMajors {
		if supported == major {
			return true
		}
	}
	return false
}

func SupportedNodeMajorsCopy() []string {
	out := make([]string, len(SupportedNodeMajors))
	for i, major := range SupportedNodeMajors {
		out[i] = strconv.Itoa(major)
	}
	return out
}
