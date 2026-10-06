// Package projectmodel defines deterministic, offline, whole-repository
// structural facts about a codebase. It may depend on pkg/semantics for
// source parsing, but it never imports pkg/codesignal or any GitHub-related
// package, so a consumer that only needs raw project facts never pulls in
// analysis policy or a GitHub client.
package projectmodel
