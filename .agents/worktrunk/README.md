# Optional Worktrunk bridge

Worktrunk manages branches and Git worktrees; Continuity supplies the shared project context, task claims, and integration record. Worktrunk is optional: all Continuity workflows continue to work with plain Git.

## Enable the orientation hook

Install the `continuity` CLI on your `PATH`, review `wt.toml.example`, then copy its contents into `.config/wt.toml`. Worktrunk asks for approval before it runs project hooks. The `pre-start` hook only prints Continuity's bounded project orientation and live ownership status in a newly created worktree; it never starts a claim automatically.

Claims stay explicit because a claim needs the real harness worker identity and a narrow code scope. Guessing either could let the wrong session write or block unrelated work. For harnesses with automatic session hooks, open the harness in the new worktree before claiming it so its displayed session identity can be used.

## Parallel worker flow

1. Create a dedicated branch and worktree: `wt switch --create agent-auth`.
2. Start the harness in that worktree, then use that run's real owner ID (for example, `claude:agent-auth`): `continuity context --owner <harness:session-id> --scope internal/auth` and `continuity status`.
3. Run `continuity ready`, then claim the selected task with that same owner. For a planned task: `continuity start --id T-001 --owner <harness:session-id> --selection-reason "highest-priority ready outcome; unblocks the next task"`. Its goal, acceptance, validation, and focused scope come from the task contract by default.
4. Start the harness in that worktree. Use one worktree and one branch for each writer.
5. Commit and run `continuity finalize` before `wt merge main` or opening a pull request.
6. After the worker commit reaches the integration branch, run `continuity integrate --id T-001`, then remove the finished worktree.

If no queued task exists, use an unclaimed integration worktree. In a legacy project, use `continuity task add --approved ...`. In an approved plan, an agent may use `continuity task add --derived --outcome O-001 --scope path --validates command --rationale text ...` only for a focused technical step inside that active outcome; the command immediately commits the queue record. Neither route invents product work.

Do not share a branch between active writers. Continuity now rejects a second live writer claim on the same named branch even if their folders and scopes differ.
