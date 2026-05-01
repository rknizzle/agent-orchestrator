import os
import sys
from dotenv import load_dotenv
from github_client import GitHubClient
from config_manager import ConfigManager

# Load environment variables
load_dotenv()

def show_board():
    # Load config relative to current directory if not specified otherwise
    config_manager = ConfigManager(".")
    is_valid, missing = config_manager.validate_required()
    if not is_valid:
        print(f"[!] Missing configuration: {', '.join(missing)}")
        sys.exit(1)

    token = config_manager.get("GITHUB_TOKEN")
    project_id = config_manager.get("GITHUB_PROJECT_ID")
    status_field_id = config_manager.get("GITHUB_STATUS_FIELD_ID")
    
    print("[*] Fetching Project Board...")
    try:
        gh_client = GitHubClient(token, project_id, status_field_id)
    except Exception as e:
        print(f"[!] Failed to initialize: {e}")
        sys.exit(1)

    # Re-using the logic from get_all_actionable_tasks but without the status filter
    query = """
    query($projectId: ID!) {
      node(id: $projectId) {
        ... on ProjectV2 {
          items(first: 100) {
            nodes {
              fieldValueByName(name: "Status") {
                ... on ProjectV2ItemFieldSingleSelectValue {
                  name
                }
              }
              content {
                ... on Issue {
                  title
                  number
                  url
                }
              }
            }
          }
        }
      }
    }
    """
    
    try:
        data = gh_client._run_query(query, {"projectId": project_id})
        items = data.get("data", {}).get("node", {}).get("items", {}).get("nodes", [])
    except Exception as e:
        print(f"[!] Error fetching data: {e}")
        sys.exit(1)

    print(f"\n{'ID':<6} | {'STATUS':<30} | {'TITLE'}")
    print("-" * 90)

    found = False
    # Sort items by number if possible
    valid_items = []
    for item in items:
        if not item or not item.get("content"): continue
        
        # Skip items that have no status
        status_value = item.get("fieldValueByName")
        if not status_value:
            continue
            
        status = status_value.get("name", "No Status")
        content = item["content"]
        valid_items.append({
            "number": content["number"],
            "status": status,
            "title": content["title"]
        })
    
    for row in sorted(valid_items, key=lambda x: x['number']):
        status_str = row['status']
        # Highlight AI WORKING to make it stand out
        if status_str == "AI WORKING":
            status_str = f"\033[93m{status_str}\033[0m" # Yellow
        elif "READY" in status_str or "DONE" in status_str:
            status_str = f"\033[92m{status_str}\033[0m" # Green
        elif "FOLLOW UP" in status_str:
            status_str = f"\033[91m{status_str}\033[0m" # Red

        print(f"#{row['number']:<5} | {status_str:<30} | {row['title']}")
        found = True

    if not found:
        print("No issues found in this project.")
    print("")

if __name__ == "__main__":
    show_board()
