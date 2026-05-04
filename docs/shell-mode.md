# Proposal: `--shell` Flag for Interactive Issue Context

## Overview
The `--shell` flag allows developers to immediately jump into a local, interactive environment pre-configured with the context of a specific GitHub issue. This bridges the gap between the automated orchestrator and manual development, enabling quick troubleshooting, manual adjustments, or deep-dive learning.

## Usage
```bash
orchestrator --repo-path ~/Dev/my-repo --issue 123 --shell
```

## How It Works

### 1. Context Fetching
The orchestrator identifies the targeted issue and fetches:
- Issue Title and Body.
- All comments (to understand the conversation history).
- Relevant labels and current project status.
- Linked Pull Request (if any).

### 2. Environment Preparation
The orchestrator performs the following setup automatically:
- **Worktree Creation:** Checks out a temporary worktree for the issue's branch. If no branch exists, it creates one based on the issue name/number.
- **Context Injection:** Generates a `.context.md` (or `ISSUE_CONTEXT.md`) file in the root of the worktree. This file serves as a "brain dump" of everything the agent and developer need to know about the task.
- **Prompt Preparation:** Pre-calculates the same system prompts that would be used by the AI agent during automated phases.

### 3. Interactive Session
The orchestrator drops the user into an interactive session:
- **Sub-shell Environment:** A new sub-shell (e.g., `zsh` or `bash`) is opened in the worktree directory. The environment variables `ORCHESTRATOR_ISSUE_NUMBER` and `ORCHESTRATOR_CONTEXT_FILE` are set for use by scripts.
- **Agent CLI Integration:** 
    - The agent (e.g., `gemini-cli`) is invoked in its interactive mode.
    - Example: `gemini --interactive --system-prompt-file .context.md`
    - This allows the developer to talk directly to the agent about the issue while having the full codebase and issue context available.
- **Visual Feedback:** The shell prompt is optionally modified to indicate an active "Orchestrator Session" (e.g., `(orchestrator #123) $ `).

### 4. Lifecycle Management
- **Manual Updates:** The user can edit files, run tests, and commit changes manually.
- **Session Exit:** Upon exiting the shell (`exit` or `Ctrl+D`), the orchestrator detects if changes were made.
- **Synchronization:** The orchestrator asks if the user wants to:
    - Push changes to the remote branch.
    - Update the GitHub Project status (e.g., move from `AI WORKING` back to `AI TODO` or `AI READY TO IMPLEMENT`).
    - Post a summary comment to the GitHub issue.

## Implementation Details

### CLI Interface
Update `cmd/orchestrator/main.go` to support:
```go
shellFlag := flag.Bool("shell", false, "Open an interactive shell with the context of the target issue.")
// Must be used with --issue
```

### Context Generation Utility
A new function in `internal/agent` will generate the context file:
```go
func CreateContextFile(task github.Task, path string) error {
    // Write Issue title, body, comments, and labels to a Markdown file
    // This file acts as the 'Short-term memory' for the agent session
}
```

### Interactive Execution
Use the `syscall` or `os/exec` package to transfer control to the user's shell:
```go
cmd := exec.Command(os.Getenv("SHELL"))
cmd.Dir = worktreePath
cmd.Stdin = os.Stdin
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
cmd.Run()
```

## Benefits
- **Zero Setup Time:** No need to manually find the issue, checkout the branch, or read through GitHub comments to get up to speed.
- **Context Parity:** The developer sees exactly the same context the AI agent sees.
- **Hybrid Workflow:** Easily hand off tasks from AI to human and back again.
