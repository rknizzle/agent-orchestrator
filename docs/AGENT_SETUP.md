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

#### Programmatically Create Statuses
Use the `updateProjectV2SingleSelectFieldOptions` mutation to ensure all required statuses exist. 
**Note**: This mutation replaces the list, so fetch existing options first if you wish to preserve them.

```graphql
mutation {
  updateProjectV2SingleSelectFieldOptions(
    input: {
      projectId: "PROJECT_ID"
      fieldId: "FIELD_ID"
      options: [
        { name: "AI BRAINSTORM", color: "BLUE" },
        { name: "AI TODO", color: "GRAY" },
        { name: "AI WORKING", color: "YELLOW" },
        { name: "AI FOLLOW UP QUESTIONS", color: "ORANGE" },
        { name: "AI FOLLOW UP QUESTIONS ANSWERED", color: "GREEN" },
        { name: "AI READY TO PLAN", color: "BLUE" },
        { name: "AI PLAN NEEDS REVIEW", color: "PURPLE" },
        { name: "AI PLAN FEEDBACK", color: "RED" },
        { name: "AI READY TO IMPLEMENT", color: "GREEN" },
        { name: "AI PR READY", color: "GREEN" },
        { name: "AI REVIEWING PR", color: "PURPLE" },
        { name: "AI PR REVIEW FEEDBACK", color: "RED" }
      ]
    }
  ) {
    field { ... on ProjectV2SingleSelectField { id } }
  }
}
```

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
- **Concurrency**: Immediately sets status to `AI WORKING` to lock tasks.
- **Isolation**: Creates a temporary Git **Worktree** for each task.
- **State Machine**: Agent output must include `<NEXT_STATE>` (one of the statuses above) and `<COMMENT>`.
