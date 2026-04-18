# manifest

Reserved directory for the `manifest` command domain — operator-facing manifest authoring, inspection, and validation workflows that live under Forge.

No code lives here yet. The first implementation ticket replaces this README with real Go source files that expose a `Command() *cobra.Command` constructor following the pattern documented in [`ARCHITECTURE.md`](../../ARCHITECTURE.md).

- Owning ticket: [MK-4](https://linear.app/wiscotrashpanda/issue/MK-4/set-up-go-workspace-and-internal-package-structure) reserves this directory; the domain implementation lands in a follow-up MK ticket.
- Boundary: schema types, kind constants, and schema-oriented validation belong in `alloy`. Forge's `manifest` domain wraps those primitives in operator-facing commands.
