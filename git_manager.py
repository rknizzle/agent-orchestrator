import os
import subprocess

def setup_worktree(repo_path: str, issue_number: int) -> str:
    """Creates a git worktree for the specific issue."""
    worktrees_dir = f"{os.path.abspath(repo_path)}-worktrees"
    os.makedirs(worktrees_dir, exist_ok=True)
    worktree_path = os.path.join(worktrees_dir, f"issue-{issue_number}")
    branch_name = f"issue-{issue_number}"
    
    if os.path.exists(worktree_path):
        print(f"[*] Worktree already exists at {worktree_path}")
        return worktree_path

    # Check if branch exists
    try:
        subprocess.run(["git", "rev-parse", "--verify", branch_name], cwd=repo_path, capture_output=True, check=True)
        branch_exists = True
    except subprocess.CalledProcessError:
        branch_exists = False

    print("[*] Ensuring main repository is on the latest default branch...")
    try:
        # Try to checkout main or master
        try:
            subprocess.run(["git", "checkout", "main"], cwd=repo_path, capture_output=True, check=True)
            default_branch = "main"
        except subprocess.CalledProcessError:
            subprocess.run(["git", "checkout", "master"], cwd=repo_path, capture_output=True, check=True)
            default_branch = "master"
            
        # Pull the latest changes
        subprocess.run(["git", "pull", "origin", default_branch], cwd=repo_path, capture_output=True, check=True)
        print(f"[*] Successfully updated '{default_branch}' to the latest commit.")
    except subprocess.CalledProcessError as e:
        print(f"[!] Warning: Could not update main repository. It may have uncommitted changes or be offline.")

    print(f"[*] Creating git worktree at {worktree_path} for branch {branch_name}...")
    try:
        if branch_exists:
            subprocess.run(["git", "worktree", "add", worktree_path, branch_name], cwd=repo_path, check=True)
        else:
            subprocess.run(["git", "worktree", "add", "-b", branch_name, worktree_path], cwd=repo_path, check=True)
    except subprocess.CalledProcessError as e:
        print(f"[!] Failed to create worktree: {e}")
        raise

    return worktree_path

def cleanup_worktree(repo_path: str, worktree_path: str):
    """Removes the git worktree."""
    print(f"[*] Cleaning up worktree at {worktree_path}...")
    try:
        subprocess.run(["git", "worktree", "remove", "--force", worktree_path], cwd=repo_path, check=True)
    except subprocess.CalledProcessError as e:
        print(f"[!] Failed to remove worktree: {e}")

def create_pull_request(repo_path: str, issue_title: str, issue_number: int, pr_description: str = "") -> str:
    """Uses the GitHub CLI to create a Pull Request from the issue's branch."""
    branch_name = f"issue-{issue_number}"
    print(f"[*] Creating Pull Request for branch {branch_name}...")
    
    body = f"Resolves #{issue_number}\n\n"
    if pr_description:
        body += f"### Agent Summary:\n{pr_description}"
    
    try:
        # Try to create the PR using the gh cli
        result = subprocess.run(
            ["gh", "pr", "create", "--title", issue_title, "--body", body, "--head", branch_name],
            cwd=repo_path,
            capture_output=True,
            text=True,
            check=True
        )
        pr_url = result.stdout.strip()
        print(f"[*] PR successfully created: {pr_url}")
        return pr_url
    except subprocess.CalledProcessError as e:
        # If it fails, check if a PR already exists for this branch
        if "already exists" in e.stderr:
            print("[*] PR already exists for this branch. Fetching its URL...")
            result = subprocess.run(
                ["gh", "pr", "view", branch_name, "--json", "url", "--jq", ".url"],
                cwd=repo_path,
                capture_output=True,
                text=True,
                check=True
            )
            pr_url = result.stdout.strip()
            print(f"[*] Found existing PR: {pr_url}")
            return pr_url
            
        print(f"[!] Failed to create PR: {e.stderr}")
        return None

def post_pr_comment(repo_path: str, issue_number: int, comment: str):
    """Uses the GitHub CLI to post a comment on the Pull Request associated with the issue branch."""
    branch_name = f"issue-{issue_number}"
    print(f"[*] Posting comment to PR for branch {branch_name}...")
    
    try:
        subprocess.run(
            ["gh", "pr", "comment", branch_name, "--body", comment],
            cwd=repo_path,
            capture_output=True,
            text=True,
            check=True
        )
        print(f"[*] Successfully posted comment to PR.")
    except subprocess.CalledProcessError as e:
        print(f"[!] Failed to post comment to PR: {e.stderr}")
