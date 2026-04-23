import argparse
import os
import sys
import time
import subprocess
import json
from dotenv import load_dotenv

from local_client import LocalClient
from agent import process_task
from git_manager import setup_worktree, cleanup_worktree, create_pull_request, post_pr_comment

# Load environment variables
load_dotenv()

VALID_STATUSES = [
    "AI BRAINSTORM",
    "AI TODO",
    "AI FOLLOW UP QUESTIONS ANSWERED",
    "AI PLAN FEEDBACK",
    "AI READY TO IMPLEMENT",
    "AI PR REVIEW FEEDBACK",
    "PENDING EXTERNAL REVIEW"
]

LOCKED_STATUS = "AI WORKING"

def parse_arguments():
    parser = argparse.ArgumentParser(description="Local Agent Orchestrator")
    parser.add_argument(
        "--status",
        choices=VALID_STATUSES,
        help="The specific ticket status the orchestrator should look for to process once."
    )
    parser.add_argument(
        "--repo-path",
        required=True,
        help="The local path to the target git repository."
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=60,
        help="Polling interval in seconds for daemon mode (default: 60)."
    )
    return parser.parse_args()

def check_pr_status(repo_path, branch_name):
    """Checks the status of GitHub checks for a PR."""
    try:
        result = subprocess.run(
            ["gh", "pr", "checks", branch_name, "--json", "conclusion,status"],
            cwd=repo_path,
            capture_output=True,
            text=True,
            check=True
        )
        checks = json.loads(result.stdout)
        if not checks:
            return "PENDING"
            
        if all(c.get("conclusion") == "success" for c in checks):
            return "SUCCESS"
        
        if any(c.get("conclusion") in ["failure", "cancelled", "timed_out"] for c in checks):
            return "FAILURE"
            
        return "PENDING"
    except Exception as e:
        print(f"[!] Error checking PR status: {e}")
        return "PENDING"

def get_pr_comments(repo_path, branch_name):
    """Fetches review comments from a PR."""
    try:
        result = subprocess.run(
            ["gh", "pr", "view", branch_name, "--json", "comments,reviews"],
            cwd=repo_path,
            capture_output=True,
            text=True,
            check=True
        )
        data = json.loads(result.stdout)
        feedback = []
        for review in data.get("reviews", []):
            if review.get("body"):
                feedback.append(f"Review by {review['author']['login']}: {review['body']}")
        for comment in data.get("comments", []):
             feedback.append(f"Comment by {comment['author']['login']}: {comment['body']}")
        return "\n\n".join(feedback)
    except Exception as e:
        print(f"[!] Error fetching PR comments: {e}")
        return "Could not fetch detailed feedback."

def print_agent_context(task):
    print("\n--- Agent Context ---")
    print(f"Title: {task['issue_title']}")
    print(f"Body Content:\n{task['issue_body']}")
    if task.get('issue_comments'):
        print(f"\nRecent Comments ({len(task['issue_comments'])}):")
        print("\n---\n".join(task['issue_comments']))
    print("---------------------\n")

def run_orchestration_loop(client, task, target_status, repo_path):
    # Special handling for External Review polling
    if target_status == "PENDING EXTERNAL REVIEW":
        branch_name = f"issue-{task['issue_number']}"
        print(f"[*] Checking external review status for {branch_name}...")
        status = check_pr_status(repo_path, branch_name)
        
        if status == "SUCCESS":
            print("[*] External review PASSED.")
            client.update_item_status(task['project_item_id'], "AI PR READY")
            client.post_comment(task['project_item_id'], "External AI Review PASSED. PR is ready for merge.")
        elif status == "FAILURE":
            print("[*] External review FAILED.")
            feedback = get_pr_comments(repo_path, branch_name)
            client.update_item_status(task['project_item_id'], "AI PR REVIEW FEEDBACK")
            client.post_comment(task['project_item_id'], f"External AI Review FAILED. Feedback collected from GitHub:\n\n{feedback}")
        else:
            print("[*] External review still pending...")
        return

    # Normal Agent Task Processing
    print(f"[*] Locking task by setting status to '{LOCKED_STATUS}'...")
    try:
        current_file = client.update_item_status(task['project_item_id'], LOCKED_STATUS)
        task['project_item_id'] = current_file # Update ID as file path changed
        print(f"[*] Task successfully locked.")
    except Exception as e:
        print(f"[!] Failed to lock task: {e}")
        sys.exit(1)

    print_agent_context(task)

    try:
        worktree_path = setup_worktree(repo_path, task['issue_number'])
    except Exception as e:
        print(f"[!] Failed to set up worktree: {e}")
        sys.exit(1)

    try:
        print(f"[*] Handing task over to Gemini CLI (cwd: {worktree_path})...")
        next_status, agent_comment = process_task(target_status, task, cwd=worktree_path)
    finally:
        cleanup_worktree(repo_path, worktree_path)
    
    # Handle PR creation/updates
    if next_status == "AI PR READY":
        if target_status == "AI PR REVIEW FEEDBACK":
            # Just post comment to PR, then wait for review again
            post_pr_comment(repo_path, task['issue_number'], f"**🤖 Local Agent Updated Implementation:**\n\n{agent_comment}")
            next_status = "PENDING EXTERNAL REVIEW"
        else:
            pr_url = create_pull_request(repo_path, task['issue_title'], task['issue_number'], pr_description=agent_comment)
            if pr_url:
                agent_comment += f"\n\n**Pull Request:** {pr_url}"
                next_status = "PENDING EXTERNAL REVIEW"
            else:
                print("[!] Failed to create PR.")

    print(f"\n--- Agent Response ---")
    print(agent_comment)
    print(f"----------------------\n")
    
    if agent_comment:
        print(f"[*] Logging agent response to task file...")
        client.post_comment(task['project_item_id'], agent_comment)
    
    print(f"[*] Updating task status to '{next_status}'...")
    client.update_item_status(task['project_item_id'], next_status)
    print(f"[*] Task status successfully updated.")

def main():
    args = parse_arguments()
    target_status = args.status
    repo_path = os.path.abspath(args.repo_path)
    interval = args.interval

    if not os.path.isdir(os.path.join(repo_path, ".git")):
        print(f"[!] Error: '{repo_path}' is not a valid git repository.", file=sys.stderr)
        sys.exit(1)

    print(f"[*] Starting Local Agent Orchestrator")
    client = LocalClient(os.path.dirname(os.path.abspath(__file__)))

    if target_status:
        print(f"[*] Searching for task with status: '{target_status}'...")
        task = client.get_first_item_by_status(target_status)
        if not task:
            print(f"[*] No tasks found with status '{target_status}'.")
            sys.exit(0)
        run_orchestration_loop(client, task, target_status, repo_path)
    else:
        print("[*] Starting polling loop...")
        try:
            status_index = 0
            empty_cycles = 0
            while True:
                status = VALID_STATUSES[status_index]
                task = client.get_first_item_by_status(status)
                if task:
                    print(f"\n[*] Found Task in '{status}': {task['issue_title']}")
                    run_orchestration_loop(client, task, status, repo_path)
                    empty_cycles = 0
                else:
                    empty_cycles += 1
                
                status_index = (status_index + 1) % len(VALID_STATUSES)
                if empty_cycles >= len(VALID_STATUSES):
                    time.sleep(interval)
                    empty_cycles = 0
        except KeyboardInterrupt:
            print("\n[*] Stopped by user.")
            sys.exit(0)

if __name__ == "__main__":
    main()
