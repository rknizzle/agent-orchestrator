import os
import re
import argparse

def parse_frontmatter(content):
    match = re.match(r'^---\s*\n(.*?)\n---\s*\n', content, re.DOTALL)
    if match:
        frontmatter_raw = match.group(1)
        metadata = {}
        for line in frontmatter_raw.split('\n'):
            if ':' in line:
                key, val = line.split(':', 1)
                metadata[key.strip()] = val.strip().strip('"').strip("'")
        return metadata
    return {}

def show_board(base_dir):
    orchestrator_dir = os.path.join(os.path.abspath(base_dir), ".orchestrator")
    tasks_dir = os.path.join(orchestrator_dir, "tasks")
    folders = ["new", "active", "pending_review", "blocked", "completed"]
    
    if not os.path.exists(tasks_dir):
        print(f"[!] Error: .orchestrator directory not found in {base_dir}")
        return

    print(f"\n{'ID':<8} | {'STATUS':<25} | {'TITLE'}")
    print("-" * 80)
    
    found_tasks = False
    for folder in folders:
        folder_path = os.path.join(tasks_dir, folder)
        if not os.path.exists(folder_path):
            continue
            
        for f in sorted(os.listdir(folder_path)):
            if f.endswith(".md"):
                file_path = os.path.join(folder_path, f)
                try:
                    with open(file_path, 'r') as tf:
                        content = tf.read()
                    metadata = parse_frontmatter(content)
                    
                    task_id = metadata.get("id", "N/A")
                    status = metadata.get("status", "N/A")
                    title = metadata.get("title", "Untitled")
                    
                    print(f"{task_id:<8} | {status:<25} | {title}")
                    found_tasks = True
                except Exception as e:
                    print(f"[!] Failed to parse {f}: {e}")

    if not found_tasks:
        print("No tasks found.")
    print("")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Local Agent Task Board")
    parser.add_argument("--dir", default=".", help="Directory containing .orchestrator folder")
    args = parser.parse_args()
    show_board(args.dir)
