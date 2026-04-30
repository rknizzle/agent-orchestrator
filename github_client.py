import os
import requests

class GitHubClient:
    def __init__(self, token: str, project_id: str, status_field_id: str):
        self.token = token
        self.project_id = project_id
        self.status_field_id = status_field_id
        self.headers = {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json",
            "GraphQL-Features": "projects_v2_queries"
        }
        self.status_options = self._fetch_status_options()

    def _run_query(self, query: str, variables: dict = None) -> dict:
        response = requests.post(
            "https://api.github.com/graphql",
            json={"query": query, "variables": variables or {}},
            headers=self.headers
        )
        if response.status_code != 200:
            raise Exception(f"GraphQL request failed with status {response.status_code}: {response.text}")
        
        data = response.json()
        if "errors" in data:
            raise Exception(f"GraphQL errors: {data['errors']}")
            
        return data

    def _fetch_status_options(self) -> dict:
        """Fetches the available status options and returns a map of {name: id}."""
        query = """
        query($projectId: ID!) {
          node(id: $projectId) {
            ... on ProjectV2 {
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
        """
        data = self._run_query(query, {"projectId": self.project_id})
        
        fields = data.get("data", {}).get("node", {}).get("fields", {}).get("nodes", [])
        status_map = {}
        for field in fields:
            if field and field.get("id") == self.status_field_id:
                for option in field.get("options", []):
                    status_map[option["name"]] = option["id"]
                break
                
        if not status_map:
            raise Exception("Could not fetch status options. Check your PROJECT_ID and STATUS_FIELD_ID.")
            
        return status_map

    def get_all_actionable_tasks(self, valid_statuses: list[str]):
        """Fetches all project items that have one of the valid statuses."""
        query = """
        query($projectId: ID!) {
          node(id: $projectId) {
            ... on ProjectV2 {
              items(first: 100) {
                nodes {
                  id
                  status: fieldValueByName(name: "Status") {
                    ... on ProjectV2ItemFieldSingleSelectValue {
                      name
                    }
                  }
                  branch: fieldValueByName(name: "Branch") {
                    ... on ProjectV2ItemFieldTextValue {
                      text
                    }
                  }
                  content {
                    ... on Issue {
                      id
                      title
                      body
                      url
                      number
                      repository {
                        nameWithOwner
                      }
                      labels(first: 10) {
                        nodes {
                          name
                        }
                      }
                      comments(last: 10) {
                        nodes {
                          author {
                            login
                          }
                          body
                        }
                      }
                    }
                  }
                }
              }
            }
          }
        }
        """
        data = self._run_query(query, {"projectId": self.project_id})
        items = data.get("data", {}).get("node", {}).get("items", {}).get("nodes", [])
        
        actionable_tasks = []
        for item in items:
            if not item: continue
            
            status_node = item.get("status")
            if status_node and status_node.get("name") in valid_statuses:
                content = item.get("content")
                if content:
                    comments = []
                    for comment_node in content.get("comments", {}).get("nodes", []):
                        if not comment_node: continue
                        author = comment_node.get("author", {}).get("login", "Unknown") if comment_node.get("author") else "Unknown"
                        comments.append(f"@{author}:\n{comment_node['body']}")

                    labels = [label["name"] for label in content.get("labels", {}).get("nodes", []) if label]

                    # Use custom Branch field if provided, otherwise default to issue number
                    branch_node = item.get("branch")
                    custom_branch = branch_node.get("text") if branch_node else None
                    branch_name = custom_branch if custom_branch else f"issue-{content['number']}"

                    actionable_tasks.append({
                        "project_item_id": item["id"],
                        "issue_node_id": content["id"],
                        "issue_title": content["title"],
                        "issue_body": content["body"],
                        "issue_comments": comments,
                        "issue_labels": labels,
                        "issue_url": content["url"],
                        "issue_number": content["number"],
                        "repo_name": content["repository"]["nameWithOwner"],
                        "current_status": status_node.get("name"),
                        "branch_name": branch_name
                    })
        return actionable_tasks

    def update_item_status(self, project_item_id: str, new_status_name: str):
        """Updates the status of a project item."""
        if new_status_name not in self.status_options:
            raise ValueError(f"Status '{new_status_name}' is not a valid option. Valid options: {list(self.status_options.keys())}")
            
        option_id = self.status_options[new_status_name]
        
        mutation = """
        mutation($projectId: ID!, $itemId: ID!, $fieldId: ID!, $optionId: String!) {
          updateProjectV2ItemFieldValue(
            input: {
              projectId: $projectId,
              itemId: $itemId,
              fieldId: $fieldId,
              value: {
                singleSelectOptionId: $optionId
              }
            }
          ) {
            projectV2Item {
              id
            }
          }
        }
        """
        variables = {
            "projectId": self.project_id,
            "itemId": project_item_id,
            "fieldId": self.status_field_id,
            "optionId": option_id
        }
        
        self._run_query(mutation, variables)
        return True

    def post_comment(self, issue_node_id: str, body: str):
        """Posts a comment to a GitHub Issue."""
        mutation = """
        mutation($issueId: ID!, $body: String!) {
          addComment(input: {subjectId: $issueId, body: $body}) {
            commentEdge {
              node {
                url
              }
            }
          }
        }
        """
        variables = {
            "issueId": issue_node_id,
            "body": body
        }
        
        data = self._run_query(mutation, variables)
        return data.get("data", {}).get("addComment", {}).get("commentEdge", {}).get("node", {}).get("url")
