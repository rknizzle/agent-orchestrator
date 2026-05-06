# 🤖 AI Agent Setup Guide: Agent Orchestrator

This document is designed for AI agents (like Gemini, Claude, or ChatGPT) tasked with setting up the `agent-orchestrator` for a user. It provides technical context, high-level architecture, and a step-by-step automation workflow.

---

## 🏗 System Architecture & Workflow

The `agent-orchestrator` is a state-machine based worker that bridges GitHub Issues/Projects with local AI agents.

1.  **Polling Loop**: It polls the GitHub Projects API for items with specific "AI" statuses.
2.  **Concurrency Control**: When it finds a task, it immediately sets the status to `AI WORKING` to "lock" it, preventing other orchestrator instances from picking it up.
3.  **Context Isolation**: It creates a temporary Git **Worktree** for each task. This ensures the agent works in a clean, isolated environment based on the issue's branch.
4.  **Agent Invocation**: It invokes a local AI CLI (e.g., `gemini-cli`) with a context-rich prompt (Issue body, comments, current code).
5.  **State Transition**: The agent output must include a `<NEXT_STATE>` tag. The orchestrator parses this and updates the GitHub Project item accordingly.
6.  **Cleanup**: Once the task is processed, the worktree is removed.

---

## 🛠 Prerequisites for Automation

Before you begin the setup, ensure the following are available in your environment:
- **GitHub CLI (`gh`)**: Must be authenticated.
- **Go**: Required if building from source (though downloading the release is preferred).
- **GitHub PAT**: A token with `repo` and `project` permissions.

---

## 🚀 Step-by-Step Setup Workflow

### 1. Identify GitHub IDs (Critical)
The orchestrator requires specific GraphQL IDs that are not easily found in the standard Web UI. Use the following commands to help the user find them:

```bash
# Get Project ID
gh api graphql -f query='query{user(login: "YOUR_USER"){projectV2(number: PROJECT_NUM){id}}}'

# Get Status Field ID (Look for the ID of the 'Status' field)
gh api graphql -f query='query{node(id: "PROJECT_ID"){... on ProjectV2{fields(first:20){nodes{id name}}}}}'
```

### 2. Configure GitHub Project Board
**Manual Action Required**: You must inform the user to add the following specific "Single Select" options to their Project's **Status** field:
- `AI BRAINSTORM`
- `AI TODO`
- `AI WORKING` (Used for locking)
- `AI FOLLOW UP QUESTIONS`
- `AI FOLLOW UP QUESTIONS ANSWERED`
- `AI READY TO PLAN`
- `AI PLAN NEEDS REVIEW`
- `AI PLAN FEEDBACK`
- `AI READY TO IMPLEMENT`
- `AI PR READY`
- `AI REVIEWING PR`
- `AI PR REVIEW FEEDBACK`

### 3. Install the Orchestrator
**Preferred Method**: Download the latest pre-compiled binary from GitHub Releases. This avoids environment-specific build issues.

```bash
# Find the latest release and download for the current OS/Arch
gh release download --repo rknizzle/agent-orchestrator --pattern "*Linux_x86_64.tar.gz" # Adjust pattern as needed
tar -xzf <downloaded_file>
```

### 4. Initialize Local Config
Create a `.env` file or a global `~/.orchestrator/config.yaml`. The orchestrator looks for these by default.

```yaml
GITHUB_TOKEN: "..."
GITHUB_PROJECT_ID: "..."
GITHUB_STATUS_FIELD_ID: "..."
ORCHESTRATOR_AGENT: "gemini" # or 'claude', 'cursor-agent'
```

### 5. Validate the Connection
Run a "dry run" by targeting a specific issue number without letting it loop:
```bash
./orchestrator --repo-path /path/to/repo --issue 1 --interval 0
```

---

## 💡 Pro-Tips for Agents

- **Prompt Location**: The orchestrator looks for templates in a `./prompts` directory relative to the binary. If you move the binary, ensure the `prompts/` folder follows it.
- **External Dependencies**: If the user wants to use the `gemini` agent, ensure `@google/gemini-cli` is installed via `npm`.
- **Infrastructure**: For 24/7 operation, refer to the `deploy/` directory which contains Terraform and Systemd configurations.
- **YOLO Mode**: The orchestrator invokes agents with `--yolo` flags where possible to minimize interactive hangups during automated runs.
