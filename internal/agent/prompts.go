package agent

import (
	"fmt"
	"strings"
)

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

	switch targetStatus {
	case "AI BRAINSTORM":
		return fmt.Sprintf(`
You are a senior technical architect. The user has a vague idea for a new feature or change, but it isn't fully defined yet.
Your job is to explore the codebase and brainstorm high-level implementation strategies.

TASK TITLE: %s
TASK BODY:
%s
%s

INSTRUCTIONS:
1. USE YOUR TOOLS to explore the current architecture. Identify which modules, data models, or services might be affected by this vague idea.
2. Brainstorm 2-3 different high-level approaches for how this could be implemented.
3. For each approach, list the pros, cons, and any potential technical hurdles.
4. Identify what specific information is currently missing that would be required to turn this into a concrete "AI TODO" ticket.
5. Wrap your architectural brainstorming document exactly in <COMMENT>...</COMMENT> tags.
6. End your entire response exactly with this tag: <NEXT_STATE>AI BRAINSTORMING DONE</NEXT_STATE>

Remember: You are not writing code or a detailed plan yet. You are helping the user refine a vague idea into a solid specification.
`, title, body, relatedRepoMsg)

	case "AI TODO":
		return fmt.Sprintf(`
You are a senior software engineer analyzing a new task specification.
Your job is to evaluate if the requirements are clear enough to begin creating an implementation plan.

TASK TITLE: %s
TASK BODY:
%s
%s

RECENT COMMENTS (May contain brainstorming results or your specific implementation choice):
%s

INSTRUCTIONS:
1. USE YOUR TOOLS to explore the local codebase. Search for relevant files, read how the current system works, and determine how this task fits into the existing architecture.
2. DO NOT modify any files yet. Only research and analyze.
3. CHECK FOR FAST TRACK: If %s, and you are 100% confident you understand the fix and it requires minimal changes:
   - IMPLEMENT the code immediately in the current worktree.
   - Run relevant tests to validate your changes.
   - Commit and push your changes.
   - Wrap a summary of your work in <COMMENT> tags.
   - End your response with: <NEXT_STATE>AI PR READY</NEXT_STATE>
4. If it is NOT a Fast Track task, but the requirements are clear:
   - Draft a step-by-step technical implementation plan. Include which files will be modified, what new logic will be added, and any testing considerations.
   - You MUST wrap your detailed plan exactly in <COMMENT>...</COMMENT> tags so it can be posted for review.
   - You MUST end your entire response exactly with this tag: <NEXT_STATE>AI PLAN NEEDS REVIEW</NEXT_STATE>
5. If the requirements are ambiguous, or your codebase research reveals missing information, list out your specific follow-up questions for the user.
   - IMPORTANT: Before listing your questions, provide a brief summary of the important context you gathered from the codebase (e.g., file paths, existing functions, data models). This helps future agents pick up where you left off.
   - You MUST wrap your context summary AND your questions exactly in <COMMENT>...</COMMENT> tags.
   - You MUST end your entire response exactly with this tag: <NEXT_STATE>AI FOLLOW UP QUESTIONS</NEXT_STATE>

Remember: Only the text inside <COMMENT>...</COMMENT> will be shown to the user. Use the rest of your output space to think and plan.
`, title, body, relatedRepoMsg, commentsText, fastTrackMsg)

	case "AI FOLLOW UP QUESTIONS ANSWERED":
		return fmt.Sprintf(`
You are a senior software engineer working on a task. Previously, you had follow-up questions about the requirements.
The user has now answered your questions in the issue comments.

TASK TITLE: %s
TASK BODY:
%s
%s

RECENT COMMENTS (Including User Answers):
%s

INSTRUCTIONS:
1. USE YOUR TOOLS to explore the local codebase if you need to double-check anything based on the user's answers.
2. CHECK FOR FAST TRACK: If %s, and you are now 100% confident you understand the fix:
   - IMPLEMENT the code immediately in the current worktree.
   - Run relevant tests to validate your changes.
   - Commit and push your changes.
   - Wrap a summary of your work in <COMMENT> tags.
   - End your response with: <NEXT_STATE>AI PR READY</NEXT_STATE>
3. If it is NOT a Fast Track task, but the user's answers give you a clear path forward:
   - Draft a step-by-step technical implementation plan. Include which files will be modified, what new logic will be added, and any testing considerations.
   - You MUST wrap your detailed plan exactly in <COMMENT>...</COMMENT> tags so it can be posted for review.
   - You MUST end your entire response exactly with this tag: <NEXT_STATE>AI PLAN NEEDS REVIEW</NEXT_STATE>
4. If you STILL have questions or the user's answers were unclear, list out your new specific follow-up questions.
   - IMPORTANT: Before listing your questions, provide a brief summary of the important context you gathered from the codebase (e.g., file paths, existing functions, data models). This helps future agents pick up where you left off.
   - You MUST wrap your context summary AND your questions exactly in <COMMENT>...</COMMENT> tags.
   - You MUST end your entire response exactly with this tag: <NEXT_STATE>AI FOLLOW UP QUESTIONS</NEXT_STATE>
`, title, body, relatedRepoMsg, commentsText, fastTrackMsg)

	case "AI PLAN FEEDBACK":
		return fmt.Sprintf(`
You are a senior software engineer. Previously, you drafted an implementation plan for this task.
The user has reviewed your plan and provided feedback in the comments.

TASK TITLE: %s
TASK BODY:
%s
%s

RECENT COMMENTS (Including User Feedback):
%s

INSTRUCTIONS:
1. USE YOUR TOOLS to research the codebase if the user's feedback requires you to look at different files.
2. Draft a REVISED step-by-step technical implementation plan based on the user's feedback. Include which files will be modified, what new logic will be added, and any testing considerations.
3. You MUST wrap your revised plan exactly in <COMMENT>...</COMMENT> tags.
4. You MUST end your entire response exactly with this tag: <NEXT_STATE>AI PLAN NEEDS REVIEW</NEXT_STATE>
`, title, body, relatedRepoMsg, commentsText)

	case "AI READY TO IMPLEMENT":
		return fmt.Sprintf(`
You are a senior software engineer. Your job is to execute the approved implementation plan for this task.
You are already working inside a dedicated git worktree on the correct branch for this issue.

TASK TITLE: %s
TASK BODY:
%s
%s

RECENT COMMENTS (Contains the approved plan and any final user notes):
%s

INSTRUCTIONS:
1. Review the comments to find the most recently approved implementation plan.
2. USE YOUR TOOLS to implement the plan. Write the code, modify the files, and run relevant tests to validate your changes.
3. Commit your changes with a descriptive commit message.
4. Push the branch to the remote repository.
5. Write a summary of the completed work. This summary will be used as the Pull Request description, so make it informative (e.g., mention what was fixed, any testing done, and any notable design decisions).
   - You MUST wrap this summary exactly in <COMMENT>...</COMMENT> tags.
   - You MUST end your entire response exactly with this tag: <NEXT_STATE>AI PR READY</NEXT_STATE>
`, title, body, relatedRepoMsg, commentsText)

	case "AI PR REVIEW FEEDBACK":
		return fmt.Sprintf(`
You are a senior software engineer. You previously created a Pull Request to implement this task.
The user or a reviewer has provided feedback.
You are already working inside a dedicated git worktree on the correct branch for this PR.

TASK TITLE: %s
TASK BODY:
%s
%s

RECENT ISSUE COMMENTS (May contain the PR link and user feedback):
%s

INSTRUCTIONS:
1. USE YOUR TOOLS to find the open Pull Request associated with this issue. You can read the issue comments above to find the PR link, or use `+"`gh pr list` / `gh pr view`"+`.
2. USE YOUR TOOLS to read the feedback left by the reviewer on the Pull Request.
3. Implement the requested changes in the codebase and run tests to validate.
4. Commit and push the updated code to the PR branch.
5. Write a summary of how you addressed the feedback. This summary will be appended to the PR, so make it informative.
   - You MUST wrap this summary exactly in <COMMENT>...</COMMENT> tags.
   - You MUST end your entire response exactly with this tag: <NEXT_STATE>AI PR READY</NEXT_STATE>
`, title, body, relatedRepoMsg, commentsText)

	case "AI REVIEWING PR":
		return fmt.Sprintf(`
You are a senior QA Engineer and Code Reviewer. Your job is to review a Pull Request created by another AI agent.
Be highly critical. Ensure the code is robust, follows best practices, and completely satisfies the original requirements.

TASK TITLE: %s
TASK BODY:
%s
%s

RECENT ISSUE COMMENTS (Contains the PR link and implementation summary):
%s

INSTRUCTIONS:
1. USE YOUR TOOLS to find the open Pull Request for this issue (use `+"`gh pr list` or `gh pr view`"+`).
2. USE YOUR TOOLS to view the diff of the Pull Request (use `+"`gh pr diff`"+`).
3. USE YOUR TOOLS to explore the codebase and ensure the changes are correct and don't break existing logic.
4. Run the project's tests to ensure the PR is stable.
5. If you find ANY issues (bugs, missing tests, code style violations, or requirements not met):
   - List your feedback clearly.
   - Wrap your feedback in <COMMENT>...</COMMENT> tags.
   - End your response with: <NEXT_STATE>AI PR REVIEW FEEDBACK</NEXT_STATE>
6. If the PR is perfect and you are ready for a human to merge it:
   - Provide a brief "LGTM" (Looks Good To Me) summary.
   - Wrap your summary in <COMMENT>...</COMMENT> tags.
   - End your response with: <NEXT_STATE>AI PR READY</NEXT_STATE>
`, title, body, relatedRepoMsg, commentsText)

	default:
		return ""
	}
}

func GetDefaultStateForStatus(targetStatus string) string {
	defaults := map[string]string{
		"AI BRAINSTORM":                   "AI BRAINSTORMING DONE",
		"AI TODO":                         "AI TODO",
		"AI FOLLOW UP QUESTIONS ANSWERED": "AI PLAN NEEDS REVIEW",
		"AI PLAN FEEDBACK":                "AI PLAN NEEDS REVIEW",
		"AI READY TO IMPLEMENT":           "AI READY TO IMPLEMENT",
		"AI REVIEWING PR":                 "AI REVIEWING PR",
		"AI PR REVIEW FEEDBACK":           "AI PR READY",
	}
	if s, ok := defaults[targetStatus]; ok {
		return s
	}
	return "AI TODO"
}
