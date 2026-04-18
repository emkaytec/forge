# reconcile

Reserved directory for the `reconcile` command domain — imperative entrypoints that hand work off to the reconciliation runtime.

No code lives here yet. The first implementation ticket replaces this README with real Go source files that expose a `Command() *cobra.Command` constructor following the pattern documented in [`ARCHITECTURE.md`](../../ARCHITECTURE.md).

- Owning ticket: [MK-4](https://linear.app/wiscotrashpanda/issue/MK-4/set-up-go-workspace-and-internal-package-structure) reserves this directory; the domain implementation lands in a follow-up MK ticket.
- Boundary: reconciliation runtime behavior belongs in `anvil`. This domain is the operator-facing Forge shell around that behavior, not a reimplementation of it.
