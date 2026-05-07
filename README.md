# Agent Orchestrator

A parallel GitHub Agent Orchestrator written in Go. It monitors GitHub Projects for tasks with specific statuses and invokes AI agents (like Gemini) to process them.

## Setup Instructions

### 1. Prerequisites
- **Go**: Installed on your machine.
- **GitHub CLI (`gh`)**: Authenticated with your account.
- **GitHub Personal Access Token (PAT)**: With `repo` and `project` scopes.

### 2. Configuration
The orchestrator can be configured via environment variables or a YAML config file.

#### Environment Variables
Create a `.env` file in the root directory:
```env
GITHUB_TOKEN=your_pat_here
GITHUB_PROJECT_ID=your_project_id_here
GITHUB_STATUS_FIELD_ID=your_status_field_id_here
ORCHESTRATOR_AGENT=gemini
```

#### Global Configuration
You can also use a global config file at `~/.orchestrator/config.yaml`. This allows you to set global defaults and project-specific overrides, including specifying which AI model to use for each phase:
```yaml
GITHUB_TOKEN: "your_pat_here"
GITHUB_PROJECT_ID: "your_project_id_here"
GITHUB_STATUS_FIELD_ID: "your_status_field_id_here"
ORCHESTRATOR_AGENT: "gemini"
models:
  default: "gemini-2.5-flash"
  "🤖 AI: Triage": "gemini-2.5-flash"
  "🤖 AI: Implement": "gemini-2.5-pro"
projects:
  owner/repo:
    GITHUB_PROJECT_ID: "repo_specific_project_id"
    GITHUB_STATUS_FIELD_ID: "repo_specific_field_id"
    models:
      "🤖 AI: Implement": "claude-3-5-sonnet"
```

### 3. Building and Running
To build the orchestrator:
```bash
go build -o orchestrator ./cmd/orchestrator
```

To run it:
```bash
./orchestrator --repo-path /path/to/your/repo --interval 60
```

#### CLI Flags
- `--repo-path`: (Required) Local path to the target git repository.
- `--status`: Filter for a specific ticket status (e.g., "AI TODO").
- `--issue`: Target a specific issue number only.
- `--agent`: AI agent CLI to use (overrides config).
- `--interval`: Polling interval in seconds (default: 60).

---

## 📖 Documentation
- **[AI Agent Setup Guide](docs/AGENT_SETUP.md)**: A specialized guide for AI agents to set up this orchestrator for you.
- **[Interactive Shell Mode (--shell)](docs/shell-mode.md)**: Learn how to jump into a local session with the full context of a specific issue.

## For Agents: Automatic Setup Guide

If you are an AI agent tasked with setting up this orchestrator for a user, follow these steps:

1. **Verify Environment**: Check if Go and the GitHub CLI are installed.
2. **Setup Secrets**: Ask the user for their `GITHUB_TOKEN`, `GITHUB_PROJECT_ID`, and `GITHUB_STATUS_FIELD_ID` if not already present in `.env`.
3. **Initialize Configuration**:
   - If a `.env` file doesn't exist, create one using `.env.example` as a template.
   - **Tip**: You can find your Project ID and Status Field ID using the GitHub CLI with GraphQL or by inspecting the network requests in the GitHub Projects UI.
   - Ensure the `ORCHESTRATOR_AGENT` matches the agent CLI available in the environment.
4. **Build the Binary**: Run `go build -o orchestrator ./cmd/orchestrator`.
5. **Validation**: Run a dry-poll or check connectivity by running with a specific issue number: `./orchestrator --repo-path . --issue <number> --interval 0`.
6. **Deployment**: If deploying to a server, utilize the files in the `deploy/` directory.

### Context for Agents
The orchestrator works by:
1. Polling GitHub Projects for items with statuses like `AI TODO`, `AI BRAINSTORM`, etc.
2. Checking out a temporary worktree for the issue's branch.
3. Invoking the specified AI agent with a status-specific prompt.
4. Updating the GitHub item status and posting a comment based on the agent's output.
