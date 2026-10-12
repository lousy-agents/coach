// Package domain holds project facts that both the use-case layer
// (pkg/codesignal) and the adapter that builds them (pkg/projectmodel)
// are allowed to depend on. The types live here so a use case can read
// those facts without importing the adapter.
package domain
