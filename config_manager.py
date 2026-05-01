import os
import yaml
import subprocess
import re
from pathlib import Path

class ConfigManager:
    def __init__(self, repo_path: str = None):
        self.repo_path = os.path.abspath(repo_path) if repo_path else None
        self.global_config_path = Path.home() / ".orchestrator" / "config.yaml"
        self.repo_identity = self._get_repo_identity() if self.repo_path else None
        self.config = self._load_config()

    def _get_repo_identity(self):
        """Determines the owner/repo name from the git remote."""
        try:
            result = subprocess.run(
                ["git", "remote", "get-url", "origin"],
                cwd=self.repo_path,
                capture_output=True,
                text=True,
                check=True
            )
            url = result.stdout.strip()
            # Match common git URL formats (HTTPS and SSH)
            match = re.search(r'[:/]([^/:]+/[^/.]+)(\.git)?$', url)
            if match:
                return match.group(1)
        except Exception:
            pass
        return None

    def _load_config(self):
        # 1. Start with Environment Variables
        config = {
            "GITHUB_TOKEN": os.getenv("GITHUB_TOKEN"),
            "GITHUB_PROJECT_ID": os.getenv("GITHUB_PROJECT_ID"),
            "GITHUB_STATUS_FIELD_ID": os.getenv("GITHUB_STATUS_FIELD_ID"),
            "ORCHESTRATOR_AGENT": os.getenv("ORCHESTRATOR_AGENT", "gemini"),
            "INCLUDES": []
        }

        # 2. Load Global/Central Config
        if self.global_config_path.exists():
            with open(self.global_config_path, "r") as f:
                global_data = yaml.safe_load(f) or {}

                # Apply global defaults
                for key in ["GITHUB_TOKEN", "ORCHESTRATOR_AGENT", "GITHUB_PROJECT_ID", "GITHUB_STATUS_FIELD_ID"]:
                    if key in global_data and not os.getenv(key):
                        config[key] = global_data[key]

                if "includes" in global_data:
                    config["INCLUDES"].extend(global_data["includes"])

                # Apply project-specific profile from the central config
                if self.repo_identity and "projects" in global_data:
                    project_profile = global_data["projects"].get(self.repo_identity)
                    if project_profile:
                        # Project profile overrides global/env for IDs
                        for key in ["GITHUB_PROJECT_ID", "GITHUB_STATUS_FIELD_ID", "ORCHESTRATOR_AGENT"]:
                            if key in project_profile:
                                config[key] = project_profile[key]

                        if "includes" in project_profile:
                            config["INCLUDES"].extend(project_profile["includes"])

        # 3. Legacy check for local orchestrator.yaml (still supported for backward compatibility)
        project_config_path = Path(self.repo_path) / "orchestrator.yaml" if self.repo_path else None
        if project_config_path and project_config_path.exists():
            with open(project_config_path, "r") as f:
                project_data = yaml.safe_load(f) or {}
                for key in ["GITHUB_PROJECT_ID", "GITHUB_STATUS_FIELD_ID", "ORCHESTRATOR_AGENT"]:
                    if key in project_data:
                        config[key] = project_data[key]

        return config

    def get(self, key, default=None):
        return self.config.get(key, default)

    def validate_required(self):
        required = ["GITHUB_TOKEN", "GITHUB_PROJECT_ID", "GITHUB_STATUS_FIELD_ID"]
        missing = [r for r in required if not self.config.get(r)]
        if missing:
            return False, missing
        return True, []
