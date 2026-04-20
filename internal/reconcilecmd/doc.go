// Package reconcilecmd hosts the cobra shell for
// forge reconcile local and forge reconcile remote.
//
// The planning layer, executor contracts, and per-kind handlers live
// in github.com/emkaytec/forge/internal/reconcile and its local/ and
// remote/ subpackages. This package is the operator-facing seam that
// composes those pieces into the CLI.
package reconcilecmd
