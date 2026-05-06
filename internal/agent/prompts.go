package agent

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type PromptData struct {
	Title            string
	Body             string
	CommentsText     string
	RelatedRepoMsg   string
	FastTrackMsg     string
	PRReviewComments string
}

func GetPromptForStatus(targetStatus string, task map[string]interface{}) string {
	title := task["issue_title"].(string)
	body := task["issue_body"].(string)

	comments := task["issue_comments"].([]string)
	commentsText := "No comments found."
	if len(comments) > 0 {
		commentsText = strings.Join(comments, "\n\n")
	}

	labels := task["issue_labels"].([]string)
	isSimple := false
	for _, l := range labels {
		if l == "ai-simple" {
			isSimple = true
			break
		}
	}
	if strings.Contains(title, "[SIMPLE]") || strings.Contains(body, "[SIMPLE]") {
		isSimple = true
	}

	relatedRepoMsg := ""
	if strings.Contains(body, "repo:") {
		relatedRepoMsg = "\nNOTE: This task mentions related repositories (e.g., 'repo:name'). If you need to gather context from these other local codebases, use your tools to explore those paths. If your environment has a REPOS_ROOT defined, you can find them there.\n"
	}

	fastTrackMsg := "the task title or body contains the string \"[SIMPLE]\""
	if isSimple {
		fastTrackMsg = "the requirements are clear and the task is marked as simple (the \"ai-simple\" label is present)"
	}

	prReviewComments := "No PR review feedback found yet."
	if raw, ok := task["pr_review_comments"].([]string); ok && len(raw) > 0 {
		prReviewComments = strings.Join(raw, "\n\n")
	}

	data := PromptData{
		Title:            title,
		Body:             body,
		CommentsText:     commentsText,
		RelatedRepoMsg:   relatedRepoMsg,
		FastTrackMsg:     fastTrackMsg,
		PRReviewComments: prReviewComments,
	}

	fileName := ""
	switch targetStatus {
	case "AI BRAINSTORM":
		fileName = "brainstorm.md"
	case "AI TODO":
		fileName = "todo.md"
	case "AI FOLLOW UP QUESTIONS ANSWERED":
		// Determine if we came from Brainstorm or Todo
		isBrainstorm := false
		for _, comment := range comments {
			if strings.Contains(strings.ToUpper(comment), "BRAINSTORM") {
				isBrainstorm = true
				break
			}
		}

		if isBrainstorm {
			fileName = "brainstorm.md"
		} else {
			fileName = "todo.md"
		}
	case "AI READY TO PLAN":
		fileName = "plan.md"
	case "AI PLAN FEEDBACK":
		fileName = "plan_feedback.md"
	case "AI READY TO IMPLEMENT":
		fileName = "implement.md"
	case "AI PR REVIEW FEEDBACK":
		fileName = "pr_feedback.md"
	case "AI REVIEWING PR":
		fileName = "review_pr.md"
	default:
		return ""
	}

	return executeTemplate(fileName, data)
}

func executeTemplate(fileName string, data PromptData) string {
	executablePath, _ := os.Executable()
	baseDir := filepath.Dir(executablePath)
	
	paths := []string{
		filepath.Join(baseDir, "prompts", fileName),
		filepath.Join("prompts", fileName), // Current dir fallback
	}

	var templatePath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			templatePath = p
			break
		}
	}

	if templatePath == "" {
		fmt.Printf("[!] Warning: Could not find prompt template file %s\n", fileName)
		return ""
	}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		fmt.Printf("[!] Error parsing template %s: %v\n", templatePath, err)
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		fmt.Printf("[!] Error executing template %s: %v\n", templatePath, err)
		return ""
	}

	return buf.String()
}

func GetDefaultStateForStatus(targetStatus string) string {
	defaults := map[string]string{
		"AI BRAINSTORM":                   "AI FOLLOW UP QUESTIONS",
		"AI TODO":                         "AI FOLLOW UP QUESTIONS",
		"AI FOLLOW UP QUESTIONS ANSWERED": "AI READY TO PLAN",
		"AI READY TO PLAN":                "AI PLAN NEEDS REVIEW",
		"AI PLAN FEEDBACK":                "AI PLAN NEEDS REVIEW",
		"AI READY TO IMPLEMENT":           "AI TODO", // Break infinite loop
		"AI REVIEWING PR":                 "AI PR READY", // Push to human if AI fails
		"AI PR REVIEW FEEDBACK":           "AI PR READY",
	}
	if s, ok := defaults[targetStatus]; ok {
		return s
	}
	return "AI TODO"
}
