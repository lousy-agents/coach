package main

import (
	"strconv"
)

// countingBoolFlag is a flag.Value wrapper that counts how many times Set
// was called, so parseCodesignalFlags can detect a flag supplied more than
// once (flag.FlagSet's normal Bool/String accessors silently keep only the
// last value).
type countingBoolFlag struct {
	value bool
	count int
}

func (c *countingBoolFlag) IsBoolFlag() bool { return true }

func (c *countingBoolFlag) String() string {
	if c == nil {
		return "false"
	}
	return strconv.FormatBool(c.value)
}

func (c *countingBoolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	c.value = v
	c.count++
	return nil
}

type countingStringFlag struct {
	value string
	count int
}

func (c *countingStringFlag) String() string {
	if c == nil {
		return ""
	}
	return c.value
}

func (c *countingStringFlag) Set(s string) error {
	c.value = s
	c.count++
	return nil
}
