package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func SetupWorktree(repoPath, branchName string, extraPatterns []string) (string, error) {
	worktreesDir := fmt.Sprintf("%s-worktrees", repoPath)
	if err := os.MkdirAll(worktreesDir, 0755); err != nil {
		return "", err
	}
	worktreePath := filepath.Join(worktreesDir, branchName)

	if _, err := os.Stat(worktreePath); err == nil {
		fmt.Printf("[*] Worktree already exists at %s\n", worktreePath)
		return worktreePath, nil
	}

	// Check if branch exists
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoPath
	err := cmd.Run()
	branchExists := err == nil

	fmt.Println("[*] Fetching latest changes from origin...")
	exec.Command("git", "fetch", "origin").Run() // Ignore error

	remoteBase := "origin/main"
	localDefault := "main"
	if err := exec.Command("git", "rev-parse", "--verify", "origin/main").Run(); err != nil {
		remoteBase = "origin/master"
		localDefault = "master"
	}

	// Switch main repo away if needed
	out, _ := exec.Command("git", "branch", "--show-current").Output()
	currentBranch := strings.TrimSpace(string(out))
	if currentBranch == branchName {
		fmt.Printf("[*] Branch %s is checked out in the main repo. Switching to '%s'...\n", branchName, localDefault)
		exec.Command("git", "checkout", localDefault).Run()
	}

	fmt.Printf("[*] Creating git worktree at %s for branch %s...\n", worktreePath, branchName)
	if branchExists {
		cmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
	} else {
		fmt.Printf("[*] Starting new branch from %s...\n", remoteBase)
		cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, remoteBase)
	}
	cmd.Dir = repoPath
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create worktree: %v, output: %s", err, string(out))
	}

	if err := SyncUntrackedFiles(repoPath, worktreePath, extraPatterns); err != nil {
		fmt.Printf("[!] Warning: Failed to sync untracked files: %v\n", err)
	}

	return worktreePath, nil
}

func SyncUntrackedFiles(repoPath, worktreePath string, extraPatterns []string) error {
	patterns := extraPatterns
	if len(patterns) == 0 {
		patterns = []string{".env*"}
	}

	fmt.Printf("[*] Syncing untracked files using patterns: %s\n", strings.Join(patterns, ", "))

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(repoPath, pattern))
		for _, source := range matches {
			relPath, _ := filepath.Rel(repoPath, source)
			destination := filepath.Join(worktreePath, relPath)

			fi, err := os.Stat(source)
			if err != nil {
				continue
			}

			if fi.IsDir() {
				os.RemoveAll(destination)
				exec.Command("cp", "-R", source, destination).Run()
			} else {
				os.MkdirAll(filepath.Dir(destination), 0755)
				exec.Command("cp", source, destination).Run()
			}
		}
	}
	return nil
}

func CleanupWorktree(repoPath, worktreePath string) {
	fmt.Printf("[*] Cleaning up worktree at %s...\n", worktreePath)
	exec.Command("git", "worktree", "remove", "--force", worktreePath).Run()
}

func CreatePullRequest(repoPath, issueTitle string, issueNumber int, branchName, repoName, prDescription string) (string, error) {
	fmt.Printf("[*] Creating Pull Request for branch %s...\n", branchName)
	issueRef := fmt.Sprintf("#%d", issueNumber)
	if repoName != "" {
		issueRef = fmt.Sprintf("%s#%d", repoName, issueNumber)
	}
	body := fmt.Sprintf("Resolves %s\n\n### Agent Summary:\n%s", issueRef, prDescription)

	cmd := exec.Command("gh", "pr", "create", "--title", issueTitle, "--body", body, "--head", branchName)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "already exists") {
			fmt.Println("[*] PR already exists for this branch. Fetching its URL...")
			cmd = exec.Command("gh", "pr", "view", branchName, "--json", "url", "--jq", ".url")
			cmd.Dir = repoPath
			out, _ = cmd.Output()
			return strings.TrimSpace(string(out)), nil
		}
		return "", fmt.Errorf("failed to create PR: %s", string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

func PostPRComment(repoPath, branchName, comment string) error {
	fmt.Printf("[*] Posting comment to PR for branch %s...\n", branchName)
	cmd := exec.Command("gh", "pr", "comment", branchName, "--body", comment)
	cmd.Dir = repoPath
	return cmd.Run()
}
