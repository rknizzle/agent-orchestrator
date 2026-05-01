import os
import subprocess
import shutil
import glob

def setup_worktree(repo_path: str, branch_name: str, extra_patterns: list[str] = None) -> str:
    """Creates a git worktree for the specific branch and syncs necessary untracked files."""
    worktrees_dir = f"{os.path.abspath(repo_path)}-worktrees"
    os.makedirs(worktrees_dir, exist_ok=True)
    worktree_path = os.path.join(worktrees_dir, branch_name)
    
    if os.path.exists(worktree_path):
        print(f"[*] Worktree already exists at {worktree_path}")
        return worktree_path

    # Check if branch exists
    try:
        subprocess.run(["git", "rev-parse", "--verify", branch_name], cwd=repo_path, capture_output=True, check=True)
        branch_exists = True
    except subprocess.CalledProcessError:
        branch_exists = False

    print("[*] Fetching latest changes from origin...")
    try:
        subprocess.run(["git", "fetch", "origin"], cwd=repo_path, capture_output=True, check=True)
        
        # Determine the remote default branch (try main then master)
        try:
            subprocess.run(["git", "rev-parse", "--verify", "origin/main"], cwd=repo_path, capture_output=True, check=True)
            remote_base = "origin/main"
            local_default = "main"
        except subprocess.CalledProcessError:
            subprocess.run(["git", "rev-parse", "--verify", "origin/master"], cwd=repo_path, capture_output=True, check=True)
            remote_base = "origin/master"
            local_default = "master"

        # If the branch is currently checked out in the main repo, git worktree add will fail.
        # Switch main repo away from the branch if needed.
        current_branch = subprocess.run(["git", "branch", "--show-current"], cwd=repo_path, capture_output=True, text=True, check=True).stdout.strip()
        if current_branch == branch_name:
            print(f"[*] Branch {branch_name} is checked out in the main repo. Switching main repo to '{local_default}'...")
            subprocess.run(["git", "checkout", local_default], cwd=repo_path, capture_output=True, check=True)

    except subprocess.CalledProcessError as e:
        print(f"[!] Warning: Could not fetch or determine remote base. Proceeding with local state.")
        remote_base = "HEAD" # Fallback

    print(f"[*] Creating git worktree at {worktree_path} for branch {branch_name}...")
    try:
        if branch_exists:
            # If it exists, just add the worktree for that branch
            subprocess.run(["git", "worktree", "add", worktree_path, branch_name], cwd=repo_path, check=True)
        else:
            # If NEW, create branch explicitly from the latest remote state
            print(f"[*] Starting new branch from {remote_base}...")
            subprocess.run(["git", "worktree", "add", "-b", branch_name, worktree_path, remote_base], cwd=repo_path, check=True)
    except subprocess.CalledProcessError as e:
        print(f"[!] Failed to create worktree: {e}")
        raise

    # Sync untracked files/directories to the worktree
    sync_untracked_files(repo_path, worktree_path, extra_patterns=extra_patterns)

    return worktree_path

def sync_untracked_files(repo_path: str, worktree_path: str, extra_patterns: list[str] = None):
    """
    Copies untracked/ignored files from the main repo to the worktree.
    Uses extra_patterns (from central config), otherwise defaults to .env*
    """
    patterns = set(extra_patterns) if extra_patterns else set()
    
    # Always include .env* if no patterns are provided at all
    if not patterns:
        patterns.add(".env*")
    
    print(f"[*] Syncing untracked files using patterns: {', '.join(patterns)}")
    
    for pattern in patterns:
        source_matches = glob.glob(os.path.join(repo_path, pattern))
        for source in source_matches:
            # Calculate relative path to maintain directory structure if necessary
            rel_path = os.path.relpath(source, repo_path)
            destination = os.path.join(worktree_path, rel_path)
            
            try:
                if os.path.isdir(source):
                    if os.path.exists(destination):
                        shutil.rmtree(destination)
                    shutil.copytree(source, destination)
                    print(f"[*] Synced directory: {rel_path}")
                else:
                    os.makedirs(os.path.dirname(destination), exist_ok=True)
                    shutil.copy2(source, destination)
                    print(f"[*] Synced file: {rel_path}")
            except Exception as e:
                print(f"[!] Failed to sync {rel_path}: {e}")

def cleanup_worktree(repo_path: str, worktree_path: str):
    """Removes the git worktree."""
    print(f"[*] Cleaning up worktree at {worktree_path}...")
    try:
        subprocess.run(["git", "worktree", "remove", "--force", worktree_path], cwd=repo_path, check=True)
    except subprocess.CalledProcessError as e:
        print(f"[!] Failed to remove worktree: {e}")

def create_pull_request(repo_path: str, issue_title: str, issue_number: int, branch_name: str, repo_name: str = None, pr_description: str = "") -> str:
    """Uses the GitHub CLI to create a Pull Request from the issue's branch."""
    print(f"[*] Creating Pull Request for branch {branch_name}...")
    
    # Use fully qualified issue reference if repo_name is provided
    issue_ref = f"{repo_name}#{issue_number}" if repo_name else f"#{issue_number}"
    body = f"Resolves {issue_ref}\n\n"
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

def post_pr_comment(repo_path: str, branch_name: str, comment: str):
    """Uses the GitHub CLI to post a comment on the Pull Request associated with the branch."""
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
