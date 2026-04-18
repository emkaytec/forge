# initcmd

Reserved directory for the `init` command domain — bootstrap and initialization workflows exposed as `forge init ...` subcommands.

The directory is named `initcmd` rather than `init` because Go reserves the identifier `init` for package initializer functions, which makes `package init` illegal. The cobra command itself is still named `init`; only the Go package name differs.

No code lives here yet. The first implementation ticket replaces this README with real Go source files that expose a `Command() *cobra.Command` constructor following the pattern documented in [`ARCHITECTURE.md`](../../ARCHITECTURE.md).

- Owning ticket: [MK-4](https://linear.app/wiscotrashpanda/issue/MK-4/set-up-go-workspace-and-internal-package-structure) reserves this directory; the domain implementation lands in a follow-up MK ticket.
