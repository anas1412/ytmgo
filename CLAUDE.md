# CLAUDE.md

Behavioral guidelines to reduce common LLM coding mistakes. Merge with project-specific instructions as needed.

**Tradeoff:** These guidelines bias toward caution over speed. For trivial tasks, use judgment.

## 1. Think Before Coding

**Don't assume. Don't hide confusion. Surface tradeoffs.**

Before implementing:
- State your assumptions explicitly. If uncertain, ask.
- If multiple interpretations exist, present them - don't pick silently.
- If a simpler approach exists, say so. Push back when warranted.
- If something is unclear, stop. Name what's confusing. Ask.

## 2. Simplicity First

**Minimum code that solves the problem. Nothing speculative.**

- No features beyond what was asked.
- No abstractions for single-use code.
- No "flexibility" or "configurability" that wasn't requested.
- No error handling for impossible scenarios.
- If you write 200 lines and it could be 50, rewrite it.

Ask yourself: "Would a senior engineer say this is overcomplicated?" If yes, simplify.

## 3. Surgical Changes

**Touch only what you must. Clean up only your own mess.**

When editing existing code:
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor things that aren't broken.
- Match existing style, even if you'd do it differently.
- If you notice unrelated dead code, mention it - don't delete it.

When your changes create orphans:
- Remove imports/variables/functions that YOUR changes made unused.
- Don't remove pre-existing dead code unless asked.

The test: Every changed line should trace directly to the user's request.

## 4. Goal-Driven Execution

**Define success criteria. Loop until verified.**

Transform tasks into verifiable goals:
- "Add validation" → "Write tests for invalid inputs, then make them pass"
- "Fix the bug" → "Write a test that reproduces it, then make it pass"
- "Refactor X" → "Ensure tests pass before and after"

For multi-step tasks, state a brief plan:
```
1. [Step] → verify: [check]
2. [Step] → verify: [check]
3. [Step] → verify: [check]
```

Strong success criteria let you loop independently. Weak criteria ("make it work") require constant clarification.

---

**These guidelines are working if:** fewer unnecessary changes in diffs, fewer rewrites due to overcomplication, and clarifying questions come before implementation rather than after mistakes.

---

## ytmgo: where platform code goes

The TUI is Linux only; Windows gets the GUI (see `win_migration.md`).
The shared packages under `internal/` serve both, so they must not care
which OS they run on.

- **OS-specific code lives in `internal/proc`, `internal/deps`, or
  `internal/settings`** (or a new package of the same kind), never in
  the shared packages or the front ends.
  - `proc`: starting and killing external tools.
  - `deps`: finding and fetching them.
  - `settings`: data directories and path handling.
- **Each OS gets its native way, not an imitation of another.** Windows
  uses Job Objects, `%LOCALAPPDATA%`, `.exe`, and `CREATE_NO_WINDOW`; Linux
  uses process groups and XDG. No POSIX shims or emulation.
- **Code that does not compile elsewhere goes in `_windows.go` / `_unix.go`
  files.** Code that only differs in a value uses `runtime.GOOS`.
- **Run every external tool through `proc.CommandContext` and `deps.Path`,**
  never `exec.Command` with a bare name. On Windows a bare name finds
  nothing, and a child started without `proc` flashes a console window
  in the GUI.
- **Windows tests run locally under Wine:**
  `GOOS=windows go test -c -o x.exe ./internal/<pkg>`, then
  `wine x.exe -test.v` from the package directory. Tests that isolate
  user dirs must fake `LOCALAPPDATA` and `USERPROFILE` as well as `HOME`
  and `XDG_*`.
