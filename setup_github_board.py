import argparse
import os
import sys
import requests
from dotenv import load_dotenv, set_key

def main():
    parser = argparse.ArgumentParser(description="Fetch GitHub Project V2 IDs")
    parser.add_argument("owner", help="GitHub username or organization name")
    parser.add_argument("project_number", type=int, help="The project board number (found in the URL: github.com/users/<owner>/projects/<NUMBER>)")
    args = parser.parse_args()

    load_dotenv()
    token = os.getenv("GITHUB_TOKEN")
    
    if not token:
        print("[!] Error: GITHUB_TOKEN not found in .env")
        sys.exit(1)

    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    # GraphQL query to get the project ID and the 'Status' field options
    query = """
    query($owner: String!, $number: Int!) {
      repositoryOwner(login: $owner) {
        ... on ProjectV2Owner {
          projectV2(number: $number) {
            id
            title
            fields(first: 20) {
              nodes {
                ... on ProjectV2SingleSelectField {
                  id
                  name
                  options {
                    id
                    name
                  }
                }
              }
            }
          }
        }
      }
    }
    """
    
    variables = {
        "owner": args.owner,
        "number": args.project_number
    }

    response = requests.post(
        "https://api.github.com/graphql",
        json={"query": query, "variables": variables},
        headers=headers
    )

    if response.status_code != 200:
        print(f"[!] API Request Failed: {response.status_code}")
        print(response.text)
        sys.exit(1)

    data = response.json()
    if "errors" in data:
        print("[!] GraphQL Errors:")
        for error in data["errors"]:
            print(f"  - {error['message']}")
        sys.exit(1)

    owner_data = data.get("data", {}).get("repositoryOwner")
    if not owner_data or not owner_data.get("projectV2"):
        print(f"[!] Could not find Project #{args.project_number} for owner '{args.owner}'.")
        print("Please ensure the project number is correct and the token has 'project' scope.")
        sys.exit(1)

    project = owner_data["projectV2"]
    project_id = project["id"]
    print(f"[*] Found Project: {project['title']}")
    print(f"[*] Project Node ID: {project_id}")

    status_field = None
    for field in project.get("fields", {}).get("nodes", []):
        if field and field.get("name") == "Status":
            status_field = field
            break

    if not status_field:
        print("[!] Could not find a single-select field named 'Status' on this project board.")
        sys.exit(1)

    status_field_id = status_field["id"]
    print(f"[*] Found 'Status' Field ID: {status_field_id}")
    
    print("\n[*] Available Status Options:")
    for option in status_field.get("options", []):
        print(f"  - {option['name']} (ID: {option['id']})")

    # Update .env file automatically
    env_file = ".env"
    set_key(env_file, "GITHUB_PROJECT_ID", project_id)
    set_key(env_file, "GITHUB_STATUS_FIELD_ID", status_field_id)
    
    print(f"\n[*] Successfully updated {env_file} with the Project ID and Status Field ID!")

if __name__ == "__main__":
    main()