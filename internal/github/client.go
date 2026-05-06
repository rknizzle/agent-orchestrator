package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type GitHubClient struct {
	Token         string
	ProjectID     string
	StatusFieldID string
	StatusOptions map[string]string
}

type Task struct {
	ProjectItemID    string
	IssueNodeID      string
	IssueTitle       string
	IssueBody        string
	IssueComments    []string
	IssueLabels      []string
	IssueURL         string
	IssueNumber      int
	RepoName         string
	CurrentStatus    string
	BranchName       string
	PRReviewDecision string
	PRReviewComments []string
}

func NewGitHubClient(token, projectID, statusFieldID string) (*GitHubClient, error) {
	client := &GitHubClient{
		Token:         token,
		ProjectID:     projectID,
		StatusFieldID: statusFieldID,
	}
	err := client.FetchStatusOptions()
	return client, err
}

func (c *GitHubClient) query(query string, variables map[string]interface{}, v interface{}) error {
	reqBody, err := json.Marshal(map[string]interface{}{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("GraphQL-Features", "projects_v2_queries")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("GraphQL request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return err
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("GraphQL errors: %v", gqlResp.Errors)
	}

	return json.Unmarshal(gqlResp.Data, v)
}

func (c *GitHubClient) FetchStatusOptions() error {
	var resp struct {
		Node struct {
			Fields struct {
				Nodes []struct {
					ID      string `json:"id"`
					Name    string `json:"name"`
					Options []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"options"`
				} `json:"nodes"`
			} `json:"fields"`
		} `json:"node"`
	}

	query := `
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
    `
	if err := c.query(query, map[string]interface{}{"projectId": c.ProjectID}, &resp); err != nil {
		return err
	}

	statusMap := make(map[string]string)
	for _, field := range resp.Node.Fields.Nodes {
		if field.ID == c.StatusFieldID {
			for _, option := range field.Options {
				statusMap[option.Name] = option.ID
			}
			break
		}
	}

	if len(statusMap) == 0 {
		return fmt.Errorf("could not fetch status options. Check your PROJECT_ID and STATUS_FIELD_ID")
	}

	c.StatusOptions = statusMap
	return nil
}

func (c *GitHubClient) GetAllActionableTasks(validStatuses []string) ([]Task, error) {
	var resp struct {
		Node struct {
			Items struct {
				Nodes []struct {
					ID     string `json:"id"`
					Status *struct {
						Name string `json:"name"`
					} `json:"status"`
					Branch *struct {
						Text string `json:"text"`
					} `json:"branch"`
					Content *struct {
						ID         string `json:"id"`
						Title      string `json:"title"`
						Body       string `json:"body"`
						URL        string `json:"url"`
						Number     int    `json:"number"`
						Repository struct {
							NameWithOwner string `json:"nameWithOwner"`
						} `json:"repository"`
						PullRequests struct {
							Nodes []struct {
								ID             string `json:"id"`
								URL            string `json:"url"`
								ReviewDecision string `json:"reviewDecision"`
								Reviews        struct {
									Nodes []struct {
										State  string `json:"state"`
										Body   string `json:"body"`
										Author *struct {
											Login string `json:"login"`
										} `json:"author"`
										Comments struct {
											Nodes []struct {
												Body string `json:"body"`
											} `json:"nodes"`
										} `json:"comments"`
									} `json:"nodes"`
								} `json:"reviews"`
							} `json:"nodes"`
						} `json:"pullRequests"`
						Labels struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
						Comments struct {
							Nodes []struct {
								Author *struct {
									Login string `json:"login"`
								} `json:"author"`
								Body string `json:"body"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"content"`
				} `json:"nodes"`
			} `json:"items"`
		} `json:"node"`
	}

	query := `
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
                      pullRequests(first: 1, states: [OPEN]) {
                        nodes {
                          id
                          url
                          reviewDecision
                          reviews(last: 5) {
                            nodes {
                              state
                              body
                              author { login }
                              comments(last: 5) {
                                nodes {
                                  body
                                }
                              }
                            }
                          }
                        }
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
    `
	if err := c.query(query, map[string]interface{}{"projectId": c.ProjectID}, &resp); err != nil {
		return nil, err
	}

	var actionableTasks []Task
	statusSet := make(map[string]bool)
	for _, s := range validStatuses {
		statusSet[s] = true
	}

	for _, node := range resp.Node.Items.Nodes {
		if node.Status == nil || !statusSet[node.Status.Name] || node.Content == nil {
			continue
		}

		var comments []string
		for _, commentNode := range node.Content.Comments.Nodes {
			author := "Unknown"
			if commentNode.Author != nil {
				author = commentNode.Author.Login
			}
			comments = append(comments, fmt.Sprintf("@%s:\n%s", author, commentNode.Body))
		}

		// Parse PR Review Info
		prDecision := ""
		var prReviewComments []string
		if len(node.Content.PullRequests.Nodes) > 0 {
			pr := node.Content.PullRequests.Nodes[0]
			prDecision = pr.ReviewDecision
			for _, review := range pr.Reviews.Nodes {
				if review.Body != "" {
					author := "Unknown"
					if review.Author != nil {
						author = review.Author.Login
					}
					prReviewComments = append(prReviewComments, fmt.Sprintf("Review by @%s (%s):\n%s", author, review.State, review.Body))
				}
				for _, c := range review.Comments.Nodes {
					prReviewComments = append(prReviewComments, fmt.Sprintf("PR Comment: %s", c.Body))
				}
			}
		}

		var labels []string
		for _, labelNode := range node.Content.Labels.Nodes {
			labels = append(labels, labelNode.Name)
		}

		branchName := fmt.Sprintf("issue-%d", node.Content.Number)
		if node.Branch != nil {
			branchName = node.Branch.Text
		}

		actionableTasks = append(actionableTasks, Task{
			ProjectItemID:    node.ID,
			IssueNodeID:      node.Content.ID,
			IssueTitle:       node.Content.Title,
			IssueBody:        node.Content.Body,
			IssueComments:    comments,
			IssueLabels:      labels,
			IssueURL:         node.Content.URL,
			IssueNumber:      node.Content.Number,
			RepoName:         node.Content.Repository.NameWithOwner,
			CurrentStatus:    node.Status.Name,
			BranchName:       branchName,
			PRReviewDecision: prDecision,
			PRReviewComments: prReviewComments,
		})
	}

	return actionableTasks, nil
}

func (c *GitHubClient) GetTaskByNumber(issueNumber int) (*Task, error) {
	// We can just reuse GetAllActionableTasks but with a filter if we want, 
	// or we can write a dedicated query. A dedicated query is cleaner.
	var resp struct {
		Node struct {
			Items struct {
				Nodes []struct {
					ID     string `json:"id"`
					Status *struct {
						Name string `json:"name"`
					} `json:"status"`
					Branch *struct {
						Text string `json:"text"`
					} `json:"branch"`
					Content *struct {
						ID         string `json:"id"`
						Title      string `json:"title"`
						Body       string `json:"body"`
						URL        string `json:"url"`
						Number     int    `json:"number"`
						Repository struct {
							NameWithOwner string `json:"nameWithOwner"`
						} `json:"repository"`
						PullRequests struct {
							Nodes []struct {
								ID             string `json:"id"`
								URL            string `json:"url"`
								ReviewDecision string `json:"reviewDecision"`
								Reviews        struct {
									Nodes []struct {
										State  string `json:"state"`
										Body   string `json:"body"`
										Author *struct {
											Login string `json:"login"`
										} `json:"author"`
										Comments struct {
											Nodes []struct {
												Body string `json:"body"`
											} `json:"nodes"`
										} `json:"comments"`
									} `json:"nodes"`
								} `json:"reviews"`
							} `json:"nodes"`
						} `json:"pullRequests"`
						Labels struct {
							Nodes []struct {
								Name string `json:"name"`
							} `json:"nodes"`
						} `json:"labels"`
						Comments struct {
							Nodes []struct {
								Author *struct {
									Login string `json:"login"`
								} `json:"author"`
								Body string `json:"body"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"content"`
				} `json:"nodes"`
			} `json:"items"`
		} `json:"node"`
	}

	query := `
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
                      pullRequests(first: 1, states: [OPEN]) {
                        nodes {
                          id
                          url
                          reviewDecision
                          reviews(last: 5) {
                            nodes {
                              state
                              body
                              author { login }
                              comments(last: 5) {
                                nodes {
                                  body
                                }
                              }
                            }
                          }
                        }
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
    `
	if err := c.query(query, map[string]interface{}{"projectId": c.ProjectID}, &resp); err != nil {
		return nil, err
	}

	for _, node := range resp.Node.Items.Nodes {
		if node.Content != nil && node.Content.Number == issueNumber {
			var comments []string
			for _, commentNode := range node.Content.Comments.Nodes {
				author := "Unknown"
				if commentNode.Author != nil {
					author = commentNode.Author.Login
				}
				comments = append(comments, fmt.Sprintf("@%s:\n%s", author, commentNode.Body))
			}

			prDecision := ""
			var prReviewComments []string
			if len(node.Content.PullRequests.Nodes) > 0 {
				pr := node.Content.PullRequests.Nodes[0]
				prDecision = pr.ReviewDecision
				for _, review := range pr.Reviews.Nodes {
					if review.Body != "" {
						author := "Unknown"
						if review.Author != nil {
							author = review.Author.Login
						}
						prReviewComments = append(prReviewComments, fmt.Sprintf("Review by @%s (%s):\n%s", author, review.State, review.Body))
					}
					for _, c := range review.Comments.Nodes {
						prReviewComments = append(prReviewComments, fmt.Sprintf("PR Comment: %s", c.Body))
					}
				}
			}

			var labels []string
			for _, labelNode := range node.Content.Labels.Nodes {
				labels = append(labels, labelNode.Name)
			}

			branchName := fmt.Sprintf("issue-%d", node.Content.Number)
			if node.Branch != nil {
				branchName = node.Branch.Text
			}

			statusName := ""
			if node.Status != nil {
				statusName = node.Status.Name
			}

			return &Task{
				ProjectItemID:    node.ID,
				IssueNodeID:      node.Content.ID,
				IssueTitle:       node.Content.Title,
				IssueBody:        node.Content.Body,
				IssueComments:    comments,
				IssueLabels:      labels,
				IssueURL:         node.Content.URL,
				IssueNumber:      node.Content.Number,
				RepoName:         node.Content.Repository.NameWithOwner,
				CurrentStatus:    statusName,
				BranchName:       branchName,
				PRReviewDecision: prDecision,
				PRReviewComments: prReviewComments,
			}, nil
		}
	}

	return nil, fmt.Errorf("issue #%d not found in project", issueNumber)
}

func (c *GitHubClient) UpdateItemStatus(projectItemID, newStatusName string) error {
	optionID, ok := c.StatusOptions[newStatusName]
	if !ok {
		return fmt.Errorf("status '%s' is not a valid option", newStatusName)
	}

	mutation := `
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
    `
	variables := map[string]interface{}{
		"projectId": c.ProjectID,
		"itemId":    projectItemID,
		"fieldId":   c.StatusFieldID,
		"optionId":  optionID,
	}

	var resp struct{}
	return c.query(mutation, variables, &resp)
}

func (c *GitHubClient) PostComment(issueNodeID, body string) (string, error) {
	var resp struct {
		AddComment struct {
			CommentEdge struct {
				Node struct {
					URL string `json:"url"`
				} `json:"node"`
			} `json:"commentEdge"`
		} `json:"addComment"`
	}

	mutation := `
        mutation($issueId: ID!, $body: String!) {
          addComment(input: {subjectId: $issueId, body: $body}) {
            commentEdge {
              node {
                url
              }
            }
          }
        }
    `
	variables := map[string]interface{}{
		"issueId": issueNodeID,
		"body":    body,
	}

	if err := c.query(mutation, variables, &resp); err != nil {
		return "", err
	}

	return resp.AddComment.CommentEdge.Node.URL, nil
}
