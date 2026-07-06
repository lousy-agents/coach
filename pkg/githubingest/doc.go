// Package githubingest reads individual file contents from a GitHub
// repository via the GitHub Contents API, authenticated as a GitHub App
// installation.
//
// This package is optional and separately packaged from pkg/semantics:
// pkg/githubingest depends on github.com/google/go-github and
// github.com/bradleyfalzon/ghinstallation/v2, and pkg/semantics never
// imports pkg/githubingest or these dependencies. Consumers that only
// analyze raw source bytes never need to build or vendor a GitHub client.
package githubingest
