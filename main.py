import argparse
import os
import sys
import time
import multiprocessing
import re
from dotenv import load_dotenv

from github_client import GitHubClient
from agent import process_task
from git_manager import setup_worktree, cleanup_worktree, create_pull_request, post_pr_comment
from config_manager import ConfigManager

# Load environment variables from .env file
load_dotenv()

# Valid trigger statuses from the plan
VALID_STATUSES = [
    "AI BRAINSTORM",
    "AI TODO",
    "AI FOLLOW UP QUESTIONS ANSWERED",
    "AI PLAN FEEDBACK",
    "AI READY TO IMPLEMENT",
    "AI REVIEWING PR",
    "AI PR REVIEW FEEDBACK"
]

LOCKED_STATUS = "AI WORKING"

def parse_arguments():
    parser = argparse.ArgumentParser(description="GitHub Agent Orchestrator")
    parser.add_argument(
        "--status",
        choices=VALID_STATUSES,
        help="The specific ticket status the orchestrator should look for. If omitted, watches all actionable statuses."
    )
    parser.add_argument(
        "--issue",
        type=int,
        help="Target a specific issue number only."
    )
    parser.add_argument(
        "--repo-path",
        required=True,
        help="The local path to the target git repository."
    )
    parser.add_argument(
        "--agent",
        choices=["gemini", "claude", "cursor-agent", "agent"],
        help="The AI agent CLI to use (overrides config)."
    )
    parser.add_argument(
        "--interval",
        type=int,
        default=60,
        help="Polling interval in seconds (default: 60)."
    )
    return parser.parse_args()

def check_environment_variables():
    token = os.getenv("GITHUB_TOKEN")
    project_id = os.getenv("GITHUB_PROJECT_ID")
    status_field_id = os.getenv("GITHUB_STATUS_FIELD_ID")

    if not all([token, project_id, status_field_id]):
        print("[!] Error: Missing required environment variables.", file=sys.stderr)
        print("[!] Ensure GITHUB_TOKEN, GITHUB_PROJECT_ID, and GITHUB_STATUS_FIELD_ID are set in .env", file=sys.stderr)
        sys.exit(1)
        
    return token, project_id, status_field_id

def print_agent_context(task, prefix):
    print(f"{prefix}\n--- Agent Context ---")
    print(f"{prefix}Title: {task['issue_title']}")
    print(f"{prefix}Repo: {task['repo_name']}")
    print(f"{prefix}Body Content:\n{task['issue_body']}")
    if task.get('issue_comments'):
        print(f"{prefix}\nRecent Comments ({len(task['issue_comments'])}):")
        # Prepend prefix to each line of comments
        formatted_comments = "\n---\n".join(task['issue_comments'])
        for line in formatted_comments.split('\n'):
            print(f"{prefix}{line}")
    print(f"{prefix}---------------------\n")

def worker_main(task, target_status, repo_path, token, project_id, status_field_id, agent_type, extra_patterns):
    """Entry point for the worker process."""
    issue_num = task['issue_number']
    branch_name = task['branch_name']
    prefix = f"[#{issue_num}] "

    # Re-initialize client in the new process
    gh_client = GitHubClient(token, project_id, status_field_id)

    print(f"{prefix}[*] Locking task by setting status to '{LOCKED_STATUS}'...")
    try:
        gh_client.update_item_status(task['project_item_id'], LOCKED_STATUS)
        print(f"{prefix}[*] Task successfully locked.")
    except Exception as e:
        print(f"{prefix}[!] Failed to update status to {LOCKED_STATUS}: {e}")
        return

    print_agent_context(task, prefix)

    try:
        worktree_path = setup_worktree(repo_path, branch_name, extra_patterns=extra_patterns)
    except Exception as e:
        print(f"{prefix}[!] Failed to set up worktree. Aborting agent execution.")
        return

    try:
        print(f"{prefix}[*] Handing task over to {agent_type} CLI (cwd: {worktree_path})...")
        next_status, agent_comment = process_task(target_status, task, agent_type=agent_type, cwd=worktree_path, prefix=prefix)
    finally:
        cleanup_worktree(repo_path, worktree_path)

    # Automatically handle PR creation if the agent completed the implementation
    if next_status == "AI PR READY":
        if target_status == "AI PR REVIEW FEEDBACK":
            post_pr_comment(repo_path, branch_name, f"**🤖 Posted by Agent Orchestrator:**\n\n{agent_comment}")
            next_status = "AI REVIEWING PR"
        else:
            pr_url = create_pull_request(repo_path, task['issue_title'], issue_num, branch_name, repo_name=task['repo_name'], pr_description=agent_comment)
            if pr_url:
                agent_comment += f"\n\n**Pull Request:** {pr_url}"
                next_status = "AI REVIEWING PR"
            else:
                agent_comment += "\n\n*(Failed to automatically generate Pull Request link. Please check the branch manually.)*"


    print(f"{prefix}\n--- Agent Response ---")
    for line in agent_comment.split('\n'):
        print(f"{prefix}{line}")
    print(f"{prefix}----------------------\n")
    
    print(f"{prefix}[*] Agent determined the next status should be: '{next_status}'")
    
    if agent_comment:
        print(f"{prefix}[*] Posting agent response as a comment on Issue #{issue_num}...")
        final_comment = f"**🤖 Posted by Agent Orchestrator:**\n\n{agent_comment}"
        try:
            gh_client.post_comment(task['issue_node_id'], final_comment)
        except Exception as e:
            print(f"{prefix}[!] Failed to post comment: {e}")
    
    print(f"{prefix}[*] Updating task status on GitHub to '{next_status}'...")
    try:
        gh_client.update_item_status(task['project_item_id'], next_status)
        print(f"{prefix}[*] Task status successfully updated. Orchestration complete.")
    except Exception as e:
        print(f"{prefix}[!] Failed to update status to {next_status}: {e}")

def main():
    # Use 'spawn' for consistent behavior across platforms when using git worktrees/subprocess
    multiprocessing.set_start_method('spawn', force=True)
    
    args = parse_arguments()
    repo_path = os.path.abspath(args.repo_path)
    interval = args.interval
    target_issue = args.issue
    target_status = args.status

    if not os.path.isdir(os.path.join(repo_path, ".git")):
        print(f"[!] Error: The path '{repo_path}' is not a valid git repository.", file=sys.stderr)
        sys.exit(1)

    print(f"[*] Starting Parallel GitHub Agent Orchestrator")
    print(f"[*] Target Repository: {repo_path}")
    if target_issue:
        print(f"[*] Filtering for Issue: #{target_issue}")
    if target_status:
        print(f"[*] Filtering for Status: {target_status}")

    active_tasks = {} # issue_number -> Process

    try:
        while True:
            # 1. Prune finished processes
            finished_issues = [num for num, p in active_tasks.items() if not p.is_alive()]
            for num in finished_issues:
                del active_tasks[num]

            # 2. Re-load config in each poll to detect project changes
            config_manager = ConfigManager(repo_path)
            is_valid, missing = config_manager.validate_required()
            if not is_valid:
                print(f"[!] Missing configuration: {', '.join(missing)}")
                print(f"[!] Please set them via .env, ~/.orchestrator/config.yaml, or orchestrator.yaml in the repo.")
                time.sleep(interval)
                continue

            token = config_manager.get("GITHUB_TOKEN")
            project_id = config_manager.get("GITHUB_PROJECT_ID")
            status_field_id = config_manager.get("GITHUB_STATUS_FIELD_ID")
            agent_type = args.agent or config_manager.get("ORCHESTRATOR_AGENT")

            gh_client = GitHubClient(token, project_id, status_field_id)

            # 3. Poll for actionable tasks
            statuses_to_check = [target_status] if target_status else VALID_STATUSES
            all_tasks = gh_client.get_all_actionable_tasks(statuses_to_check)
            
            # 4. Filter by issue if requested
            if target_issue:
                all_tasks = [t for t in all_tasks if t['issue_number'] == target_issue]

            # 5. Spawn workers for new tasks
            for task in all_tasks:
                issue_num = task['issue_number']
                current_status = task['current_status']
                
                if issue_num not in active_tasks:
                    print(f"[*] Found actionable task: #{issue_num} in state '{current_status}'")
                    includes = config_manager.get("INCLUDES", [])
                    p = multiprocessing.Process(
                        target=worker_main,
                        args=(task, current_status, repo_path, token, project_id, status_field_id, agent_type, includes)
                    )
                    p.start()
                    active_tasks[issue_num] = p
            
            # 6. Sleep
            if not active_tasks:
                print(f"[*] No actionable tasks found. Sleeping for {interval} seconds...    ", end='\r')
            
            time.sleep(interval)
            
    except KeyboardInterrupt:
        print("\n[*] Shutting down orchestrator. Waiting for active workers to finish...")
        for p in active_tasks.values():
            p.join()
        print("[*] All workers finished. Exiting.")
        sys.exit(0)

if __name__ == "__main__":
    main()
