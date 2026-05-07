package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func RunAgentCLI(agentType, prompt, model, cwd, prefix string) (string, error) {
	displayType := agentType
	if len(agentType) > 0 {
		displayType = strings.ToUpper(agentType[:1]) + agentType[1:]
	}
	if model != "" {
		fmt.Printf("%s[*] Invoking %s CLI (Model: %s)... (cwd: %s)\n", prefix, displayType, model, cwd)
	} else {
		fmt.Printf("%s[*] Invoking %s CLI... (cwd: %s)\n", prefix, displayType, cwd)
	}

	executablePath, _ := os.Executable()
	policyPath := filepath.Join(filepath.Dir(executablePath), "orchestrator-policy.yaml")
	// Fallback to current dir if not found near executable
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		policyPath = "orchestrator-policy.yaml"
	}

	var cmd *exec.Cmd
	switch agentType {
	case "gemini":
		if model == "" {
			model = "auto"
		}
		cmd = exec.Command("gemini", "-p", prompt, "--model", model, "--yolo", "--policy", policyPath)
	case "claude":
		args := []string{"-p", prompt, "--bare", "--allowedTools", "Bash,Read,Edit"}
		if model != "" {
			// Claude CLI might not explicitly support --model, but passing it if supported.
			// (If it doesn't support it, this might break, so we should append if model is given)
			// Assuming standard Claude CLI usage or mapping
			args = append(args, "--model", model)
		}
		cmd = exec.Command("claude", args...)
	case "cursor-agent", "agent":
		args := []string{"-p", prompt, "--yolo"}
		if model != "" {
			args = append(args, "-m", model)
		}
		cmd = exec.Command(agentType, args...)
	default:
		return "", fmt.Errorf("unknown agent type: %s", agentType)
	}

	cmd.Dir = cwd
	cmd.Stderr = os.Stderr // Stream stderr to console

	out, err := cmd.Output()
	if err != nil {
		return string(out), err
	}

	return string(out), nil
}

func ParseAgentResponse(outputText, defaultState, prefix string) (string, string, bool) {
	success := true

	stateRegex := regexp.MustCompile(`(?is)<NEXT_STATE>(.*?)</NEXT_STATE>`)
	commentRegex := regexp.MustCompile(`(?is)<COMMENT>(.*?)</COMMENT>`)

	nextState := defaultState
	if match := stateRegex.FindStringSubmatch(outputText); len(match) > 1 {
		nextState = strings.TrimSpace(match[1])
	} else {
		fmt.Printf("%s[!] Warning: Could not find <NEXT_STATE> tag in agent output.\n", prefix)
		success = false
	}

	cleanComment := ""
	if match := commentRegex.FindStringSubmatch(outputText); len(match) > 1 {
		cleanComment = strings.TrimSpace(match[1])
	} else {
		fmt.Printf("%s[!] Warning: Could not find <COMMENT> tag in agent output.\n", prefix)
		// Fallback
		cleanComment = stateRegex.ReplaceAllString(outputText, "")
		cleanComment = strings.TrimSpace(cleanComment)
		success = false
	}

	return nextState, cleanComment, success
}

func CreateContextFile(taskMap map[string]interface{}, path string) error {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Issue Context: %v\n\n", taskMap["issue_title"]))
	sb.WriteString("## Description\n")
	sb.WriteString(fmt.Sprintf("%v\n\n", taskMap["issue_body"]))

	if comments, ok := taskMap["issue_comments"].([]string); ok && len(comments) > 0 {
		sb.WriteString("## Comments\n")
		for _, c := range comments {
			sb.WriteString(fmt.Sprintf("%s\n\n---\n\n", c))
		}
	}

	if labels, ok := taskMap["issue_labels"].([]string); ok && len(labels) > 0 {
		sb.WriteString("## Labels\n")
		sb.WriteString(strings.Join(labels, ", "))
		sb.WriteString("\n\n")
	}

	if prComments, ok := taskMap["pr_review_comments"].([]string); ok && len(prComments) > 0 {
		sb.WriteString("## PR Review Comments\n")
		for _, c := range prComments {
			sb.WriteString(fmt.Sprintf("%s\n\n---\n\n", c))
		}
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func ProcessTask(targetStatus string, task map[string]interface{}, agentType, model, cwd, prefix string) (string, string, error) {
	basePrompt := GetPromptForStatus(targetStatus, task)
	defaultState := GetDefaultStateForStatus(targetStatus)

	currentPrompt := basePrompt
	maxRetries := 2

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("%s[*] Retry Attempt %d/%d due to missing tags...\n", prefix, attempt, maxRetries)
		}

		output, err := RunAgentCLI(agentType, currentPrompt, model, cwd, prefix)
		if err != nil {
			return "", "", err
		}

		nextState, cleanComment, success := ParseAgentResponse(output, defaultState, prefix)
		if success {
			return nextState, cleanComment, nil
		}

		if attempt < maxRetries {
			currentPrompt = basePrompt + "\n\n[CRITICAL SYSTEM ERROR]: In your previous attempt, you failed to include the mandatory <COMMENT> and/or <NEXT_STATE> tags. You MUST wrap your text in <COMMENT> tags and end exactly with a <NEXT_STATE> tag as instructed above. Please try again."
		} else {
			return nextState, cleanComment, nil // Return what we have on last attempt
		}
	}

	return defaultState, "", nil
}
