# Optional Worktrunk bridge

Worktrunk manages branches and Git worktrees; Continuity supplies the shared project context, task claims, and integration record. Worktrunk is optional: all Continuity workflows continue to work with plain Git.

## Enable the orientation hook

Install the `continuity` CLI on your `PATH`, review `wt.toml.example`, then copy its contents into `.config/wt.toml`. Worktrunk asks for approval before it runs project hooks. The `pre-start` hook only prints Continuity's bounded project orientation and live ownership status in a newly created worktree; it never starts a claim automatically.

Claims stay explicit because a claim needs the real harness worker identity and a narrow code scope. Guessing either could let the wrong session write or block unrelated work. For harnesses with automatic session hooks, open the harness in the new worktree before claiming it so its displayed session identity can be used.

## Parallel worker flow

1. Create a dedicated branch and worktree: `wt switch --create agent-auth`.
2. Start the harness in that worktree, then use that run's real owner ID (for example, `claude:agent-auth`): `continuity context --owner <harness:session-id> --scope internal/auth` and `continuity status`.
3. Claim an approved task with that same owner: `continuity start --id T-001 --owner <harness:session-id> --goal ... --acceptance ... --next-action ... --scope internal/auth`.
4. Start the harness in that worktree. Use one worktree and one branch for each writer.
5. Commit and run `continuity finalize` before `wt merge main` or opening a pull request.
6. After the worker commit reaches the integration branch, run `continuity integrate --id T-001`, then remove the finished worktree.

If a user has approved a new task but no queued task exists yet, use `continuity task add --approved ...` from an unclaimed integration worktree. The command is intentionally explicit; it never invents product work.

Do not share a branch between active writers. Continuity now rejects a second live writer claim on the same named branch even if their folders and scopes differ.
