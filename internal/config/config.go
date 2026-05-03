package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type ProjectConfig struct {
	GithubProjectID     string   `yaml:"GITHUB_PROJECT_ID"`
	GithubStatusFieldID string   `yaml:"GITHUB_STATUS_FIELD_ID"`
	OrchestratorAgent   string   `yaml:"ORCHESTRATOR_AGENT"`
	DisableAIReview     bool     `yaml:"DISABLE_AI_REVIEW"`
	Includes            []string `yaml:"includes"`
}

type GlobalConfig struct {
	GithubToken         string                   `yaml:"GITHUB_TOKEN"`
	GithubProjectID     string                   `yaml:"GITHUB_PROJECT_ID"`
	GithubStatusFieldID string                   `yaml:"GITHUB_STATUS_FIELD_ID"`
	OrchestratorAgent   string                   `yaml:"ORCHESTRATOR_AGENT"`
	DisableAIReview     bool                     `yaml:"DISABLE_AI_REVIEW"`
	Includes            []string                 `yaml:"includes"`
	Projects            map[string]ProjectConfig `yaml:"projects"`
}

type Config struct {
	GithubToken         string
	GithubProjectID     string
	GithubStatusFieldID string
	OrchestratorAgent   string
	DisableAIReview     bool
	Includes            []string
	RepoPath            string
	RepoIdentity        string
}

func NewConfigManager(repoPath string) (*Config, error) {
	absRepoPath, _ := filepath.Abs(repoPath)
	cfg := &Config{
		RepoPath: absRepoPath,
	}

	cfg.RepoIdentity = getRepoIdentity(absRepoPath)
	return cfg.loadConfig()
}

func getRepoIdentity(repoPath string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	url := strings.TrimSpace(string(out))
	// Match common git URL formats (HTTPS and SSH)
	re := regexp.MustCompile(`[:/]([^/:]+/[^/.]+)(\.git)?$`)
	match := re.FindStringSubmatch(url)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (c *Config) loadConfig() (*Config, error) {
	// 1. Defaults/Env
	c.GithubToken = os.Getenv("GITHUB_TOKEN")
	c.GithubProjectID = os.Getenv("GITHUB_PROJECT_ID")
	c.GithubStatusFieldID = os.Getenv("GITHUB_STATUS_FIELD_ID")
	c.OrchestratorAgent = os.Getenv("ORCHESTRATOR_AGENT")

	if c.OrchestratorAgent == "" {
		c.OrchestratorAgent = "gemini"
	}

	// 2. Global Config
	home, _ := os.UserHomeDir()
	globalPath := filepath.Join(home, ".orchestrator", "config.yaml")
	if _, err := os.Stat(globalPath); err == nil {
		data, err := os.ReadFile(globalPath)
		if err == nil {
			var global GlobalConfig
			if err := yaml.Unmarshal(data, &global); err == nil {
				if c.GithubToken == "" {
					c.GithubToken = global.GithubToken
				}
				if c.GithubProjectID == "" {
					c.GithubProjectID = global.GithubProjectID
				}
				if c.GithubStatusFieldID == "" {
					c.GithubStatusFieldID = global.GithubStatusFieldID
				}
				if os.Getenv("ORCHESTRATOR_AGENT") == "" && global.OrchestratorAgent != "" {
					c.OrchestratorAgent = global.OrchestratorAgent
				}
				if global.DisableAIReview {
					c.DisableAIReview = true
				}
				c.Includes = append(c.Includes, global.Includes...)

				// Project specific from global
				if c.RepoIdentity != "" {
					if proj, ok := global.Projects[c.RepoIdentity]; ok {
						if proj.GithubProjectID != "" {
							c.GithubProjectID = proj.GithubProjectID
						}
						if proj.GithubStatusFieldID != "" {
							c.GithubStatusFieldID = proj.GithubStatusFieldID
						}
						if proj.OrchestratorAgent != "" {
							c.OrchestratorAgent = proj.OrchestratorAgent
						}
						if proj.DisableAIReview {
							c.DisableAIReview = true
						}
						c.Includes = append(c.Includes, proj.Includes...)
					}
				}
			}
		}
	}

	// 3. Legacy local orchestrator.yaml
	localPath := filepath.Join(c.RepoPath, "orchestrator.yaml")
	if _, err := os.Stat(localPath); err == nil {
		data, err := os.ReadFile(localPath)
		if err == nil {
			var local ProjectConfig
			if err := yaml.Unmarshal(data, &local); err == nil {
				if local.GithubProjectID != "" {
					c.GithubProjectID = local.GithubProjectID
				}
				if local.GithubStatusFieldID != "" {
					c.GithubStatusFieldID = local.GithubStatusFieldID
				}
				if local.OrchestratorAgent != "" {
					c.OrchestratorAgent = local.OrchestratorAgent
				}
				if local.DisableAIReview {
					c.DisableAIReview = true
				}
			}
		}
	}

	return c, nil
}
