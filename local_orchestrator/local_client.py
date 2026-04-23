import os
import re
from datetime import datetime

class LocalClient:
    def __init__(self, base_dir: str):
        self.base_dir = os.path.abspath(base_dir)
        self.orchestrator_dir = os.path.join(self.base_dir, ".orchestrator")
        self.tasks_dir = os.path.join(self.orchestrator_dir, "tasks")
        self.folders = ["new", "active", "pending_review", "blocked", "completed"]

    def _get_all_task_files(self):
        task_files = []
        for folder in self.folders:
            folder_path = os.path.join(self.tasks_dir, folder)
            if os.path.exists(folder_path):
                for f in os.listdir(folder_path):
                    if f.endswith(".md"):
                        task_files.append(os.path.join(folder_path, f))
        return task_files

    def _parse_frontmatter(self, content):
        match = re.match(r'^---\s*\n(.*?)\n---\s*\n', content, re.DOTALL)
        if match:
            frontmatter_raw = match.group(1)
            metadata = {}
            for line in frontmatter_raw.split('\n'):
                if ':' in line:
                    key, val = line.split(':', 1)
                    metadata[key.strip()] = val.strip().strip('"').strip("'")
            return metadata, content[match.end():]
        return {}, content

    def get_first_item_by_status(self, target_status: str):
        for file_path in self._get_all_task_files():
            try:
                with open(file_path, 'r') as f:
                    content = f.read()
            except Exception as e:
                print(f"[!] Failed to read {file_path}: {e}")
                continue

            metadata, body = self._parse_frontmatter(content)
            
            if metadata.get("status") == target_status:
                # Extract comments
                comments = []
                if "## Comments" in body:
                    parts = body.split("## Comments", 1)
                    description = parts[0].strip()
                    comments_section = parts[1]
                    # Split by headers like **@agent (2026-04-17 10:10):**
                    raw_comments = re.split(r'\n\s*\*\*@(.*?)\s*\(\d{4}-\d{2}-\d{2}\s*\d{2}:\d{2}\)\:\*\*', comments_section)
                    if len(raw_comments) > 1:
                        for i in range(1, len(raw_comments), 2):
                            author = raw_comments[i]
                            body_text = raw_comments[i+1].strip()
                            comments.append(f"@{author}:\n{body_text}")
                else:
                    description = body.strip()

                labels = [l.strip() for l in metadata.get("labels", "").split(",") if l.strip()]

                return {
                    "project_item_id": file_path,
                    "issue_node_id": file_path,
                    "issue_title": metadata.get("title", "Untitled"),
                    "issue_body": description,
                    "issue_comments": comments,
                    "issue_labels": labels,
                    "issue_url": f"file://{file_path}",
                    "issue_number": metadata.get("id", "0"),
                    "repo_name": "local"
                }
        return None

    def update_item_status(self, file_path: str, new_status_name: str):
        status_to_folder = {
            "AI TODO": "new",
            "AI WORKING": "active",
            "AI PLAN NEEDS REVIEW": "pending_review",
            "PENDING EXTERNAL REVIEW": "pending_review",
            "AI FOLLOW UP QUESTIONS": "blocked",
            "AI PR READY": "completed",
            "AI BRAINSTORMING DONE": "completed",
            "AI PR REVIEW FEEDBACK": "active"
        }
        
        target_folder = status_to_folder.get(new_status_name, "active")
        new_dir = os.path.join(self.tasks_dir, target_folder)
        os.makedirs(new_dir, exist_ok=True)
        new_file_path = os.path.join(new_dir, os.path.basename(file_path))
        
        with open(file_path, 'r') as f:
            content = f.read()
            
        if "status:" in content:
            new_content = re.sub(r'status:.*', f'status: "{new_status_name}"', content)
        else:
            new_content = content.replace("---", f"---\nstatus: \"{new_status_name}\"", 1)
        
        with open(new_file_path, 'w') as f:
            f.write(new_content)
            
        if os.path.abspath(file_path) != os.path.abspath(new_file_path):
            os.remove(file_path)
            
        return new_file_path

    def post_comment(self, file_path: str, body: str):
        # Handle cases where the file might have moved folder since the agent started
        if not os.path.exists(file_path):
            filename = os.path.basename(file_path)
            for f in self._get_all_task_files():
                if os.path.basename(f) == filename:
                    file_path = f
                    break

        with open(file_path, 'r') as f:
            content = f.read()
        
        if "## Comments" not in content:
            content += "\n\n---\n## Comments\n"
        
        now = datetime.now().strftime("%Y-%m-%d %H:%M")
        content += f"\n**@agent ({now}):**\n{body}\n"
        
        with open(file_path, 'w') as f:
            f.write(content)
            
        return f"file://{file_path}"
