# workstation

Reserved directory for the `workstation` command domain — imperative automation for local developer workstation setup.

No code lives here yet. The first implementation ticket replaces this README with real Go source files that expose a `Command() *cobra.Command` constructor following the pattern documented in [`ARCHITECTURE.md`](../../ARCHITECTURE.md).

- Owning ticket: [MK-4](https://linear.app/wiscotrashpanda/issue/MK-4/set-up-go-workspace-and-internal-package-structure) reserves this directory; the domain implementation lands in a follow-up MK ticket.
- Boundary: workstation automation belongs in Forge. Reconciliation belongs in `anvil`; shared schema belongs in `alloy`.
