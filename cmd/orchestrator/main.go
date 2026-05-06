package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/ryankennelly/agent-orchestrator/internal/agent"
	"github.com/ryankennelly/agent-orchestrator/internal/config"
	"github.com/ryankennelly/agent-orchestrator/internal/git"
	"github.com/ryankennelly/agent-orchestrator/internal/github"
)

var VALID_STATUSES = []string{
	"AI BRAINSTORM",
	"AI TODO",
	"AI FOLLOW UP QUESTIONS ANSWERED",
	"AI PLAN FEEDBACK",
	"AI READY TO IMPLEMENT",
	"AI REVIEWING PR",
	"AI PR REVIEW FEEDBACK",
}

const LOCKED_STATUS = "AI WORKING"

type Orchestrator struct {
	repoPath     string
	statusFilter string
	issueFilter  int
	agentFilter  string
	interval     time.Duration
	activeTasks  map[int]bool
	mu           sync.Mutex
}

func NewOrchestrator(repoPath, status string, issue int, agentOverride string, interval int) *Orchestrator {
	return &Orchestrator{
		repoPath:     repoPath,
		statusFilter: status,
		issueFilter:  issue,
		agentFilter:  agentOverride,
		interval:     time.Duration(interval) * time.Second,
		activeTasks:  make(map[int]bool),
	}
}

func (o *Orchestrator) Run() {
	fmt.Printf("[*] Starting Parallel GitHub Agent Orchestrator\n")
	fmt.Printf("[*] Target Repository: %s\n", o.repoPath)

	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initial poll
	o.poll()

	for {
		select {
		case <-ticker.C:
			o.poll()
		case <-sigChan:
			fmt.Println("\n[*] Shutting down orchestrator. Waiting for active workers to finish...")
			return
		}
	}
}

func (o *Orchestrator) poll() {
	cfg, err := config.NewConfigManager(o.repoPath)
	if err != nil {
		fmt.Printf("[!] Error loading config: %v\n", err)
		return
	}

	if cfg.GithubToken == "" || cfg.GithubProjectID == "" || cfg.GithubStatusFieldID == "" {
		fmt.Println("[!] Missing required configuration (GITHUB_TOKEN, GITHUB_PROJECT_ID, GITHUB_STATUS_FIELD_ID)")
		return
	}

	ghClient, err := github.NewGitHubClient(cfg.GithubToken, cfg.GithubProjectID, cfg.GithubStatusFieldID)
	if err != nil {
		fmt.Printf("[!] Failed to initialize GitHub client: %v\n", err)
		return
	}

	agentType := cfg.OrchestratorAgent
	if o.agentFilter != "" {
		agentType = o.agentFilter
	}

	statuses := VALID_STATUSES
	if o.statusFilter != "" {
		statuses = []string{o.statusFilter}
	}

	tasks, err := ghClient.GetAllActionableTasks(statuses)
	if err != nil {
		fmt.Printf("[!] Error fetching tasks: %v\n", err)
		return
	}

	o.mu.Lock()
	defer o.mu.Unlock()

	foundAny := false
	for _, task := range tasks {
		if o.issueFilter != 0 && task.IssueNumber != o.issueFilter {
			continue
		}

		if o.activeTasks[task.IssueNumber] {
			continue
		}

		// Passive Review Logic:
		// If AI reviewing is disabled and we are in the 'REVIEWING' state, 
		// we just "watch" the PR for feedback.
		if cfg.DisableAIReview && task.CurrentStatus == "AI REVIEWING PR" {
			hasFeedback := task.PRReviewDecision == "CHANGES_REQUESTED" || len(task.PRReviewComments) > 0
			if hasFeedback {
				fmt.Printf("[#%d] [*] Feedback detected on PR. Transitioning to AI PR REVIEW FEEDBACK...\n", task.IssueNumber)
				// Self-transition so the worker picks it up as a fix-it task
				ghClient.UpdateItemStatus(task.ProjectItemID, "AI PR REVIEW FEEDBACK")
				task.CurrentStatus = "AI PR REVIEW FEEDBACK"
			} else {
				// No feedback yet, keep waiting
				continue
			}
		}

		foundAny = true
		o.activeTasks[task.IssueNumber] = true
		go o.worker(task, task.CurrentStatus, ghClient, agentType, cfg.Includes)
	}

	if !foundAny && len(o.activeTasks) == 0 {
		fmt.Printf("[*] No actionable tasks found. Sleeping %v...    \r", o.interval)
	}
}

func (o *Orchestrator) worker(task github.Task, targetStatus string, ghClient *github.GitHubClient, agentType string, includes []string) {
	defer func() {
		o.mu.Lock()
		delete(o.activeTasks, task.IssueNumber)
		o.mu.Unlock()
	}()

	prefix := fmt.Sprintf("[#%d] ", task.IssueNumber)
	fmt.Printf("%s[*] Locking task by setting status to '%s'...\n", prefix, LOCKED_STATUS)

	if err := ghClient.UpdateItemStatus(task.ProjectItemID, LOCKED_STATUS); err != nil {
		fmt.Printf("%s[!] Failed to update status to %s: %v\n", prefix, LOCKED_STATUS, err)
		return
	}

	worktreePath, err := git.SetupWorktree(o.repoPath, task.BranchName, includes)
	if err != nil {
		fmt.Printf("%s[!] Failed to set up worktree: %v\n", prefix, err)
		return
	}
	defer git.CleanupWorktree(o.repoPath, worktreePath)

	taskMap := map[string]interface{}{
		"issue_title":        task.IssueTitle,
		"issue_body":         task.IssueBody,
		"issue_comments":     task.IssueComments,
		"issue_labels":       task.IssueLabels,
		"issue_number":       task.IssueNumber,
		"repo_name":          task.RepoName,
		"pr_review_comments": task.PRReviewComments,
	}

	nextStatus, agentComment, err := agent.ProcessTask(targetStatus, taskMap, agentType, worktreePath, prefix)
	if err != nil {
		fmt.Printf("%s[!] Agent process failed: %v\n", prefix, err)
		return
	}

	// Handle PR creation
	if nextStatus == "AI PR READY" {
		if targetStatus == "AI PR REVIEW FEEDBACK" {
			git.PostPRComment(o.repoPath, task.BranchName, fmt.Sprintf("**🤖 Posted by Agent Orchestrator:**\n\n%s", agentComment))
			nextStatus = "AI REVIEWING PR"
		} else {
			prURL, err := git.CreatePullRequest(o.repoPath, task.IssueTitle, task.IssueNumber, task.BranchName, task.RepoName, agentComment)
			if err == nil {
				agentComment += fmt.Sprintf("\n\n**Pull Request:** %s", prURL)
				nextStatus = "AI REVIEWING PR"
			} else {
				fmt.Printf("%s[!] Failed to create PR: %v\n", prefix, err)
				agentComment += "\n\n*(Failed to automatically generate Pull Request link. Please check the branch manually.)*"
			}
		}
	}

	fmt.Printf("%s[*] Agent determined next status: '%s'\n", prefix, nextStatus)

	if agentComment != "" {
		finalComment := fmt.Sprintf("**🤖 Posted by Agent Orchestrator:**\n\n%s", agentComment)
		if _, err := ghClient.PostComment(task.IssueNodeID, finalComment); err != nil {
			fmt.Printf("%s[!] Failed to post comment: %v\n", prefix, err)
		}
	}

	if err := ghClient.UpdateItemStatus(task.ProjectItemID, nextStatus); err != nil {
		fmt.Printf("%s[!] Failed to update status to %s: %v\n", prefix, nextStatus, err)
	} else {
		fmt.Printf("%s[*] Task status successfully updated. Orchestration complete.\n", prefix)
	}
}

func (o *Orchestrator) RunShell(issueNumber int) {
	fmt.Printf("[*] Opening interactive shell for issue #%d...\n", issueNumber)

	cfg, err := config.NewConfigManager(o.repoPath)
	if err != nil {
		fmt.Printf("[!] Error loading config: %v\n", err)
		return
	}

	ghClient, err := github.NewGitHubClient(cfg.GithubToken, cfg.GithubProjectID, cfg.GithubStatusFieldID)
	if err != nil {
		fmt.Printf("[!] Failed to initialize GitHub client: %v\n", err)
		return
	}

	task, err := ghClient.GetTaskByNumber(issueNumber)
	if err != nil {
		fmt.Printf("[!] Failed to fetch task: %v\n", err)
		return
	}

	worktreePath, err := git.SetupWorktree(o.repoPath, task.BranchName, cfg.Includes)
	if err != nil {
		fmt.Printf("[!] Failed to set up worktree: %v\n", err)
		return
	}
	defer git.CleanupWorktree(o.repoPath, worktreePath)

	taskMap := map[string]interface{}{
		"issue_title":        task.IssueTitle,
		"issue_body":         task.IssueBody,
		"issue_comments":     task.IssueComments,
		"issue_labels":       task.IssueLabels,
		"issue_number":       task.IssueNumber,
		"repo_name":          task.RepoName,
		"pr_review_comments": task.PRReviewComments,
	}

	contextFile := filepath.Join(worktreePath, ".context.md")
	if err := agent.CreateContextFile(taskMap, contextFile); err != nil {
		fmt.Printf("[!] Failed to create context file: %v\n", err)
		return
	}

	fmt.Printf("[*] Context written to %s\n", contextFile)
	fmt.Printf("[*] Entering sub-shell. Type 'exit' to return to orchestrator.\n")

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.Command(shell)
	cmd.Dir = worktreePath
	cmd.Env = append(os.Environ(), 
		fmt.Sprintf("ORCHESTRATOR_ISSUE_NUMBER=%d", issueNumber),
		fmt.Sprintf("ORCHESTRATOR_CONTEXT_FILE=%s", contextFile),
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Printf("[!] Shell exited with error: %v\n", err)
	}

	fmt.Println("[*] Exited shell. Cleaning up...")
}

func main() {
	godotenv.Load()

	statusFlag := flag.String("status", "", "The specific ticket status the orchestrator should look for.")
	issueFlag := flag.Int("issue", 0, "Target a specific issue number only.")
	repoPathFlag := flag.String("repo-path", "", "The local path to the target git repository.")
	agentFlag := flag.String("agent", "", "The AI agent CLI to use (overrides config).")
	intervalFlag := flag.Int("interval", 60, "Polling interval in seconds.")
	shellFlag := flag.Bool("shell", false, "Open an interactive shell with the context of the target issue.")

	flag.Parse()

	if *repoPathFlag == "" {
		fmt.Println("[!] Error: --repo-path is required.")
		os.Exit(1)
	}

	repoPath, _ := filepath.Abs(*repoPathFlag)
	if _, err := os.Stat(filepath.Join(repoPath, ".git")); os.IsNotExist(err) {
		fmt.Printf("[!] Error: The path '%s' is not a valid git repository.\n", repoPath)
		os.Exit(1)
	}

	orchestrator := NewOrchestrator(repoPath, *statusFlag, *issueFlag, *agentFlag, *intervalFlag)

	if *shellFlag {
		if *issueFlag == 0 {
			fmt.Println("[!] Error: --issue is required when using --shell.")
			os.Exit(1)
		}
		orchestrator.RunShell(*issueFlag)
	} else {
		orchestrator.Run()
	}
}
