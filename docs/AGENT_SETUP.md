# 🤖 AI Agent Setup Guide: Agent Orchestrator

This document is designed for AI agents (like Gemini, Claude, or ChatGPT) tasked with setting up the `agent-orchestrator` for a user. It provides technical context and a step-by-step automation workflow.

---

## 🚀 Step-by-Step Setup Workflow

### 1. Install the Orchestrator
**Preferred Method**: Download the latest pre-compiled binary from GitHub Releases to avoid build environment issues.

```bash
# Find the latest release and download for the current OS/Arch
gh release download --repo rknizzle/agent-orchestrator --pattern "*$(uname -s)_$(uname -m).tar.gz" 
tar -xzf <downloaded_file>
```

### 2. Identify & Configure GitHub Project
The orchestrator requires specific GraphQL IDs. You should fetch these and then configure the Project's "Status" field with the required options.

#### Fetch IDs
```bash
# 1. Get Project ID
gh api graphql -f query='query{user(login: "YOUR_USER"){projectV2(number: PROJECT_NUM){id}}}'

# 2. Get Status Field ID (Look for the ID of the 'Status' field)
gh api graphql -f query='query{node(id: "PROJECT_ID"){... on ProjectV2{fields(first:20){nodes{... on ProjectV2SingleSelectField{id name options{name}}}}}}}}'
```

#### Configure Statuses (Manual Action Required)
Currently, the GitHub GraphQL API does not support programmatically creating or updating the options within a Single Select field. You **must inform the user** to manually add the following exact options to their Project's "Status" field:

- `🤖 AI: Brainstorm`
- `🤖 AI: Triage`
- `⚙️ PROCESSING (Locked)`
- `👤 HUMAN: Needs Clarification`
- `🤖 AI: Review Clarification`
- `🤖 AI: Draft Plan`
- `👤 HUMAN: Review Plan`
- `🤖 AI: Revise Plan`
- `🤖 AI: Implement`
- `👤 HUMAN: Review PR`
- `🤖 AI: Review PR`
- `🤖 AI: Fix PR Feedback`

### 3. Initialize Configuration
Create a `.env` file or a global `~/.orchestrator/config.yaml`. 

```yaml
GITHUB_TOKEN: "..."
GITHUB_PROJECT_ID: "..."
GITHUB_STATUS_FIELD_ID: "..."
ORCHESTRATOR_AGENT: "gemini" # preferred agent CLI
```

### 4. Validation
Run a dry run by targeting a specific issue number with zero interval:
```bash
./orchestrator --repo-path /path/to/repo --issue 1 --interval 0
```

---

## 🏗 System Architecture Reference

- **Polling Loop**: Monitors GitHub Projects for tasks with the statuses configured above.
- **Concurrency**: Immediately sets status to `⚙️ PROCESSING (Locked)` to lock tasks.
- **Isolation**: Creates a temporary Git **Worktree** for each task.
- **State Machine**: Agent output must include `<NEXT_STATE>` (one of the statuses above) and `<COMMENT>`.
