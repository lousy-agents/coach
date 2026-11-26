// Package antipatternfixtures turns a pattern profile into anti-pattern
// fixtures in #871's layout (#872 Task 5). Each fixture is one of #871's
// catalogue patterns, scaled to a statistic the profile observed: nothing of
// the reference repository reaches it but numbers.
//
// It sits in the generation ring, which may not import the real-code ring
// that writes profiles, so the caller passes the profile schema's validator.
package antipatternfixtures
