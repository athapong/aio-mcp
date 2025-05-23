package tools

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/athapong/aio-mcp/util"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/pkg/errors"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var gitlabClient = sync.OnceValue(func() *gitlab.Client {
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		log.Fatal("GITLAB_TOKEN is required")
	}

	host := os.Getenv("GITLAB_HOST")
	if host == "" {
		log.Fatal("GITLAB_HOST is required")
	}

	client, err := gitlab.NewClient(token, gitlab.WithBaseURL(host))
	if err != nil {
		log.Fatal(errors.WithMessage(err, "failed to create gitlab client"))
	}

	return client
})

// GitLabRepoCache manages temporary cloned repositories
type GitLabRepoCache struct {
	BaseDir string
	Repos   map[string]string // map[projectPath]localPath
	mu      sync.Mutex
}

var repoCache = &GitLabRepoCache{
	BaseDir: filepath.Join(os.TempDir(), "gitlab-repos"),
	Repos:   make(map[string]string),
}

// ensureRepo ensures the repository is cloned and up-to-date
// ref can be a branch name, tag, or empty (for default branch)
func (c *GitLabRepoCache) ensureRepo(projectPath string, ref string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.MkdirAll(c.BaseDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create base directory: %v", err)
	}

	// Get repository URL and default branch
	project, _, err := gitlabClient().Projects.GetProject(projectPath, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get project: %v", err)
	}

	// If ref is empty, use default branch
	if ref == "" {
		ref = project.DefaultBranch
	}

	localPath := filepath.Join(c.BaseDir, strings.Replace(projectPath, "/", "-", -1))

	// Always clean up existing repository first
	if err := os.RemoveAll(localPath); err != nil {
		return "", fmt.Errorf("failed to clean up existing repository: %v", err)
	}
	delete(c.Repos, projectPath)

	// Ensure parent directory exists after cleanup
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create repository directory: %v", err)
	}

	cloneURL := project.HTTPURLToRepo
	token := os.Getenv("GITLAB_TOKEN")
	if token != "" {
		cloneURL = strings.Replace(cloneURL, "https://", "https://oauth2:"+token+"@", 1)
	}

	// Clone repository
	cmd := exec.Command("git", "clone", "--mirror", cloneURL, localPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to clone repository: %v", err)
	}
	c.Repos[projectPath] = localPath

	// Verify ref exists
	cmd = exec.Command("git", "-C", localPath, "rev-parse", "--verify", ref)
	if err := cmd.Run(); err != nil {
		// Clean up on failure
		os.RemoveAll(localPath)
		delete(c.Repos, projectPath)
		return "", fmt.Errorf("reference '%s' not found in repository: %v", ref, err)
	}

	return localPath, nil
}

// RegisterGitLabTool registers the GitLab tool with the MCP server
func RegisterGitLabTool(s *server.MCPServer) {
	listProjectsTool := mcp.NewTool("gitlab_list_projects",
		mcp.WithDescription("List GitLab projects"),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("gitlab group ID")),
		mcp.WithString("search", mcp.Description("Multiple terms can be provided, separated by an escaped space, either + or %20, and will be ANDed together. Example: one+two will match substrings one and two (in any order).")),
	)

	projectTool := mcp.NewTool("gitlab_get_project",
		mcp.WithDescription("Get GitLab project details"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
	)

	mrListTool := mcp.NewTool("gitlab_list_mrs",
		mcp.WithDescription("List merge requests"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("state", mcp.DefaultString("all"), mcp.Description("MR state (opened/closed/merged)")),
	)

	mrDetailsTool := mcp.NewTool("gitlab_get_mr_details",
		mcp.WithDescription("Get merge request details"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("mr_iid", mcp.Required(), mcp.Description("Merge request IID")),
	)

	mrCommentTool := mcp.NewTool("gitlab_create_MR_note",
		mcp.WithDescription("Create a note on a merge request"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("mr_iid", mcp.Required(), mcp.Description("Merge request IID")),
		mcp.WithString("comment", mcp.Required(), mcp.Description("Comment text")),
	)

	fileContentTool := mcp.NewTool("gitlab_get_file_content",
		mcp.WithDescription("Get file content from a GitLab repository"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to the file in the repository")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("Branch name, tag, or commit SHA")),
	)

	pipelineTool := mcp.NewTool("gitlab_list_pipelines",
		mcp.WithDescription("List pipelines for a GitLab project"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("status", mcp.DefaultString("all"), mcp.Description("Pipeline status (running/pending/success/failed/canceled/skipped/all)")),
	)

	commitsTool := mcp.NewTool("gitlab_list_commits",
		mcp.WithDescription("List commits in a GitLab project within a date range"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("since", mcp.Required(), mcp.Description("Start date (YYYY-MM-DD)")),
		mcp.WithString("until", mcp.Description("End date (YYYY-MM-DD). If not provided, defaults to current date")),
		mcp.WithString("ref", mcp.Required(), mcp.Description("Branch name, tag, or commit SHA")),
	)

	commitDetailsTool := mcp.NewTool("gitlab_get_commit_details",
		mcp.WithDescription("Get details of a commit"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("commit_sha", mcp.Required(), mcp.Description("Commit SHA")),
	)

	userEventsTool := mcp.NewTool("gitlab_list_user_events",
		mcp.WithDescription("List GitLab user events within a date range"),
		mcp.WithString("username", mcp.Required(), mcp.Description("GitLab username")),
		mcp.WithString("since", mcp.Required(), mcp.Description("Start date (YYYY-MM-DD)")),
		mcp.WithString("until", mcp.Description("End date (YYYY-MM-DD). If not provided, defaults to current date")),
	)

	listGroupUsersTool := mcp.NewTool("gitlab_list_group_users",
		mcp.WithDescription("List all users in a GitLab group"),
		mcp.WithString("group_id", mcp.Required(), mcp.Description("GitLab group ID")),
	)

	createMRTool := mcp.NewTool("gitlab_create_mr",
		mcp.WithDescription("Create a new merge request"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("source_branch", mcp.Required(), mcp.Description("Source branch name")),
		mcp.WithString("target_branch", mcp.Required(), mcp.Description("Target branch name")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Merge request title")),
		mcp.WithString("description", mcp.Description("Merge request description")),
	)

	cloneRepoTool := mcp.NewTool("gitlab_clone_repo",
		mcp.WithDescription("Clone or update a GitLab repository locally"),
		mcp.WithString("project_path", mcp.Required(), mcp.Description("Project/repo path")),
		mcp.WithString("ref", mcp.Description("Branch name or tag (optional, defaults to project's default branch)")),
	)

	s.AddTool(listProjectsTool, util.ErrorGuard(listProjectsHandler))
	s.AddTool(projectTool, util.ErrorGuard(getProjectHandler))
	s.AddTool(mrListTool, util.ErrorGuard(listMergeRequestsHandler))
	s.AddTool(mrDetailsTool, util.ErrorGuard(getMergeRequestHandler))
	s.AddTool(mrCommentTool, util.ErrorGuard(commentOnMergeRequestHandler))
	s.AddTool(fileContentTool, util.ErrorGuard(getFileContentHandler))
	s.AddTool(pipelineTool, util.ErrorGuard(listPipelinesHandler))
	s.AddTool(commitsTool, util.ErrorGuard(util.AdaptLegacyHandler(listCommitsHandler)))
	s.AddTool(commitDetailsTool, util.ErrorGuard(util.AdaptLegacyHandler(getCommitDetailsHandler)))
	s.AddTool(userEventsTool, util.ErrorGuard(util.AdaptLegacyHandler(listUserEventsHandler)))
	s.AddTool(listGroupUsersTool, util.ErrorGuard(util.AdaptLegacyHandler(listGroupUsersHandler)))
	s.AddTool(createMRTool, util.ErrorGuard(util.AdaptLegacyHandler(createMergeRequestHandler)))
	s.AddTool(cloneRepoTool, util.ErrorGuard(util.AdaptLegacyHandler(cloneRepoHandler)))
}

func listProjectsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	groupID := arguments["group_id"].(string)

	opt := &gitlab.ListGroupProjectsOptions{
		Archived: gitlab.Ptr(false),
		OrderBy:  gitlab.Ptr("last_activity_at"),
		Sort:     gitlab.Ptr("desc"),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	if search, ok := arguments["search"]; ok {
		opt.Search = gitlab.Ptr(search.(string))
	}

	projects, _, err := gitlabClient().Groups.ListGroupProjects(groupID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to search projects: %v", err)
	}

	var result string
	for _, project := range projects {
		result += fmt.Sprintf("ID: %d\nName: %s\nPath: %s\nDescription: %s\nLast Activity: %s\n\n",
			project.ID, project.Name, project.PathWithNamespace, project.Description, project.LastActivityAt.Format("2006-01-02 15:04:05"))
	}

	return mcp.NewToolResultText(result), nil
}

func getProjectHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	projectID := arguments["project_path"].(string)

	// Get project details
	project, _, err := gitlabClient().Projects.GetProject(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %v", err)
	}

	// Get branches
	branches, _, err := gitlabClient().Branches.ListBranches(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %v", err)
	}

	// Get tags
	tags, _, err := gitlabClient().Tags.ListTags(projectID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %v", err)
	}

	// Build basic project info
	result := fmt.Sprintf("Project Details:\nID: %d\nName: %s\nPath: %s\nDescription: %s\nURL: %s\nDefault Branch: %s\n\n",
		project.ID, project.Name, project.PathWithNamespace, project.Description, project.WebURL,
		project.DefaultBranch)

	// Add branches
	result += "Branches:\n"
	for _, branch := range branches {
		result += fmt.Sprintf("- %s\n", branch.Name)
	}

	// Add tags
	result += "\nTags:\n"
	for _, tag := range tags {
		result += fmt.Sprintf("- %s\n", tag.Name)
	}

	return mcp.NewToolResultText(result), nil
}

func listMergeRequestsHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	projectID := arguments["project_path"].(string)

	state := "all"
	if value, ok := arguments["state"]; ok {
		state = value.(string)
	}

	opt := &gitlab.ListProjectMergeRequestsOptions{
		State: gitlab.String(state),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	mrs, _, err := gitlabClient().MergeRequests.ListProjectMergeRequests(projectID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list merge requests: %v", err)
	}
	var result strings.Builder
	for _, mr := range mrs {
		result.WriteString(fmt.Sprintf("MR #%d: %s\nState: %s\nAuthor: %s\nURL: %s\nCreated: %s\n",
			mr.IID, mr.Title, mr.State, mr.Author.Username, mr.WebURL, mr.CreatedAt.Format("2006-01-02 15:04:05")))

		if mr.SourceBranch != "" {
			result.WriteString(fmt.Sprintf("Source Branch: %s\n", mr.SourceBranch))
		}
		if mr.TargetBranch != "" {
			result.WriteString(fmt.Sprintf("Target Branch: %s\n", mr.TargetBranch))
		}
		if mr.MergedAt != nil {
			result.WriteString(fmt.Sprintf("Merged At: %s\n", mr.MergedAt.Format("2006-01-02 15:04:05")))
		}
		if mr.ClosedAt != nil {
			result.WriteString(fmt.Sprintf("Closed At: %s\n", mr.ClosedAt.Format("2006-01-02 15:04:05")))
		}
		if mr.Description != "" {
			result.WriteString(fmt.Sprintf("Description: %s\n", mr.Description))
		}

		// changes
		if len(mr.Changes) > 0 {
			result.WriteString(fmt.Sprintf("Changes Count: %d\n", len(mr.Changes)))

			result.WriteString("Changes:\n")
			for _, change := range mr.Changes {
				result.WriteString(fmt.Sprintf("- Old Path: %s\n", change.OldPath))
				result.WriteString(fmt.Sprintf("  New Path: %s\n", change.NewPath))
				if change.DeletedFile {
					result.WriteString("  Status: Deleted\n")
				} else if change.NewFile {
					result.WriteString("  Status: Added\n")
				} else if change.RenamedFile {
					result.WriteString(fmt.Sprintf("  Status: Renamed from %s\n", change.OldPath))
				} else {
					result.WriteString("  Status: Modified\n")
				}
				if change.Diff != "" {
					result.WriteString("  Diff:\n")
					result.WriteString("  ```diff\n")
					result.WriteString("  " + strings.ReplaceAll(change.Diff, "\n", "\n  "))
					result.WriteString("\n  ```\n")
				}
				result.WriteString("\n")
			}
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func getMergeRequestHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	projectID := arguments["project_path"].(string)
	mrIIDStr := arguments["mr_iid"].(string)

	mrIID, err := strconv.Atoi(mrIIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid mr_iid: %v", err)
	}

	// Get MR details
	mr, _, err := gitlabClient().MergeRequests.GetMergeRequest(projectID, mrIID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request: %v", err)
	}

	// Get detailed changes
	changes, _, err := gitlabClient().MergeRequests.ListMergeRequestDiffs(projectID, mrIID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request changes: %v", err)
	}

	var result strings.Builder

	// Write MR overview
	result.WriteString(fmt.Sprintf("Merge Request #%d: %s\n", mr.IID, mr.Title))
	result.WriteString(fmt.Sprintf("Author: %s\n", mr.Author.Username))
	result.WriteString(fmt.Sprintf("Source Branch: %s\n", mr.SourceBranch))
	result.WriteString(fmt.Sprintf("Target Branch: %s\n", mr.TargetBranch))
	result.WriteString(fmt.Sprintf("State: %s\n", mr.State))
	result.WriteString(fmt.Sprintf("Created: %s\n", mr.CreatedAt.Format("2006-01-02 15:04:05")))
	// Add SHAs information
	result.WriteString(fmt.Sprintf("Base SHA: %s\n", mr.DiffRefs.BaseSha))
	result.WriteString(fmt.Sprintf("Start SHA: %s\n", mr.DiffRefs.StartSha))
	result.WriteString(fmt.Sprintf("Head SHA: %s\n\n", mr.DiffRefs.HeadSha))

	if mr.Description != "" {
		result.WriteString("Description:\n")
		result.WriteString(mr.Description)
		result.WriteString("\n\n")
	}

	// Write changes overview
	result.WriteString("Changes Overview:\n")
	result.WriteString(fmt.Sprintf("Total files changed: %d\n\n", len(changes)))

	// Write detailed changes for each file
	for _, change := range changes {
		result.WriteString(fmt.Sprintf("File: %s\n", change.NewPath))
		switch true {
		case change.NewFile:
			result.WriteString("Status: Added\n")
		case change.DeletedFile:
			result.WriteString("Status: Deleted\n")
		case change.RenamedFile:
			result.WriteString(fmt.Sprintf("Status: Renamed from %s\n", change.OldPath))
		default:
			result.WriteString("Status: Modified\n")
		}

		if change.Diff != "" {
			result.WriteString("Diff:\n")
			result.WriteString("```diff\n")
			result.WriteString(change.Diff)
			result.WriteString("\n```\n")
		}

		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func commentOnMergeRequestHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	projectID := arguments["project_path"].(string)
	mrIIDStr := arguments["mr_iid"].(string)
	comment := arguments["comment"].(string)

	mrIID, err := strconv.Atoi(mrIIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid mr_iid: %v", err)
	}

	opt := &gitlab.CreateMergeRequestNoteOptions{
		Body: gitlab.String(comment),
	}

	note, _, err := gitlabClient().Notes.CreateMergeRequestNote(projectID, mrIID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create comment: %v", err)
	}

	result := fmt.Sprintf("Comment posted successfully!\nID: %d\nAuthor: %s\nCreated: %s\nContent: %s",
		note.ID, note.Author.Username, note.CreatedAt.Format("2006-01-02 15:04:05"), note.Body)

	return mcp.NewToolResultText(result), nil
}

// Modify getFileContentHandler to use the same ref handling
func getFileContentHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	projectPath := arguments["project_path"].(string)
	filePath := arguments["file_path"].(string)
	ref := ""
	if value, ok := arguments["ref"]; ok {
		ref = value.(string)
	}

	// Ensure repository is available locally with the specified ref
	localPath, err := repoCache.ensureRepo(projectPath, ref)
	if err != nil {
		return nil, err
	}

	// Use git show to get file content
	cmd := exec.Command("git", "-C", localPath, "show", fmt.Sprintf("%s:%s", ref, filePath))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("File: %s\n", filePath))
	result.WriteString(fmt.Sprintf("Ref: %s\n", ref))
	result.WriteString("Content:\n")
	result.WriteString(string(output))

	return mcp.NewToolResultText(result.String()), nil
}

func listPipelinesHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	arguments := request.Params.Arguments
	projectID := arguments["project_path"].(string)
	status := arguments["status"].(string)

	opt := &gitlab.ListProjectPipelinesOptions{}
	if status != "all" {
		opt.Status = gitlab.Ptr(gitlab.BuildStateValue(status))
	}

	pipelines, _, err := gitlabClient().Pipelines.ListProjectPipelines(projectID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list pipelines: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Pipelines for project %s:\n\n", projectID))

	for _, pipeline := range pipelines {
		result.WriteString(fmt.Sprintf("Pipeline #%d\n", pipeline.ID))
		result.WriteString(fmt.Sprintf("Status: %s\n", pipeline.Status))
		result.WriteString(fmt.Sprintf("Ref: %s\n", pipeline.Ref))
		result.WriteString(fmt.Sprintf("SHA: %s\n", pipeline.SHA))
		result.WriteString(fmt.Sprintf("Created: %s\n", pipeline.CreatedAt.Format("2006-01-02 15:04:05")))
		result.WriteString(fmt.Sprintf("URL: %s\n\n", pipeline.WebURL))
	}

	return mcp.NewToolResultText(result.String()), nil
}

func listCommitsHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	projectID := arguments["project_path"].(string)
	since, ok := arguments["since"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: since")
	}

	until := time.Now().Format("2006-01-02")
	if value, ok := arguments["until"]; ok {
		until = value.(string)
	}

	ref := "develop"
	if value, ok := arguments["ref"]; ok {
		ref = value.(string)
	}

	sinceTime, err := time.Parse("2006-01-02", since)
	if err != nil {
		return nil, fmt.Errorf("invalid since date: %v", err)
	}

	untilTime, err := time.Parse("2006-01-02 15:04:05", until+" 23:00:00")
	if err != nil {
		return nil, fmt.Errorf("invalid until date: %v", err)
	}

	opt := &gitlab.ListCommitsOptions{
		Since:   gitlab.Ptr(sinceTime),
		Until:   gitlab.Ptr(untilTime),
		RefName: gitlab.Ptr(ref),
	}

	commits, _, err := gitlabClient().Commits.ListCommits(projectID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list commits: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Commits for project %s between %s and %s (ref: %s):\n\n",
		projectID, since, until, ref))

	for _, commit := range commits {
		result.WriteString(fmt.Sprintf("Commit: %s\n", commit.ID))
		result.WriteString(fmt.Sprintf("Author: %s\n", commit.AuthorName))
		result.WriteString(fmt.Sprintf("Date: %s\n", commit.CommittedDate.Format("2006-01-02 15:04:05")))
		result.WriteString(fmt.Sprintf("Message: %s\n", commit.Title))
		if commit.LastPipeline != nil {
			result.WriteString("Last Pipeline: \n")
			result.WriteString(fmt.Sprintf("Status: %s\n", commit.LastPipeline.Status))
			result.WriteString(fmt.Sprintf("Ref: %s\n", commit.LastPipeline.Ref))
			result.WriteString(fmt.Sprintf("SHA: %s\n", commit.LastPipeline.SHA))
			result.WriteString(fmt.Sprintf("Created: %s\n", commit.LastPipeline.CreatedAt.Format("2006-01-02 15:04:05")))

		}
	}

	return mcp.NewToolResultText(result.String()), nil
}

func getCommitDetailsHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	projectID := arguments["project_path"].(string)
	commitSHA := arguments["commit_sha"].(string)

	commit, _, err := gitlabClient().Commits.GetCommit(projectID, commitSHA, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit details: %v", err)
	}

	opt := &gitlab.GetCommitDiffOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	diffs, _, err := gitlabClient().Commits.GetCommitDiff(projectID, commitSHA, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit diffs: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Commit: %s\n", commit.ShortID))
	result.WriteString(fmt.Sprintf("Author: %s\n", commit.AuthorName))
	result.WriteString(fmt.Sprintf("Date: %s\n", commit.CommittedDate.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("Message: %s\n", commit.Title))
	result.WriteString(fmt.Sprintf("URL: %s\n\n", commit.WebURL))

	if commit.ParentIDs != nil {
		result.WriteString("Parents:\n")
		for _, parentID := range commit.ParentIDs {
			result.WriteString(fmt.Sprintf("- %s\n", parentID))
		}
		result.WriteString("\n")
	}

	result.WriteString("Diffs:\n")
	for _, diff := range diffs {
		result.WriteString(fmt.Sprintf("File: %s\n", diff.NewPath))
		result.WriteString(fmt.Sprintf("Status: %s\n", getDiffStatus(diff)))

		if diff.Diff != "" {
			result.WriteString("```diff\n")
			result.WriteString(diff.Diff)
			result.WriteString("\n```\n")
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func getDiffStatus(diff *gitlab.Diff) string {
	if diff.NewFile {
		return "Added"
	}
	if diff.DeletedFile {
		return "Deleted"
	}
	if diff.RenamedFile {
		return fmt.Sprintf("Renamed from %s", diff.OldPath)
	}
	return "Modified"
}

func listUserEventsHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	username := arguments["username"].(string)
	since, ok := arguments["since"].(string)
	if !ok {
		return nil, fmt.Errorf("missing required argument: since")
	}

	until := time.Now().Format("2006-01-02")
	if value, ok := arguments["until"]; ok {
		until = value.(string)
	}

	sinceTime, err := time.Parse("2006-01-02", since)
	if err != nil {
		return nil, fmt.Errorf("invalid since date: %v", err)
	}

	untilTime, err := time.Parse("2006-01-02 15:04:05", until+" 23:59:59")
	if err != nil {
		return nil, fmt.Errorf("invalid until date: %v", err)
	}

	opt := &gitlab.ListContributionEventsOptions{
		After:  gitlab.Ptr(gitlab.ISOTime(sinceTime)),
		Before: gitlab.Ptr(gitlab.ISOTime(untilTime)),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	events, _, err := gitlabClient().Users.ListUserContributionEvents(username, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list user events: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Events for user %s between %s and %s:\n\n",
		username, since, until))

	for _, event := range events {
		result.WriteString(fmt.Sprintf("Date: %s\n", event.CreatedAt.Format("2006-01-02 15:04:05")))
		result.WriteString(fmt.Sprintf("Action: %s\n", event.ActionName))

		if event.PushData.CommitCount != 0 {
			result.WriteString(fmt.Sprintf("Ref: %s\n", event.PushData.Ref))
			result.WriteString(fmt.Sprintf("Commit Count: %d\n", event.PushData.CommitCount))
			result.WriteString(fmt.Sprintf("Commit Title: %s\n", event.PushData.CommitTitle))
			result.WriteString(fmt.Sprintf("Commit From: %s\n", event.PushData.CommitFrom))
			result.WriteString(fmt.Sprintf("Commit To: %s\n", event.PushData.CommitTo))
		}

		if len(event.TargetType) > 0 {
			result.WriteString(fmt.Sprintf("Target Type: %s\n", event.TargetType))
		}

		if event.TargetIID != 0 {
			result.WriteString(fmt.Sprintf("Target IID: %d\n", event.TargetIID))
		}

		if event.ProjectID != 0 {
			result.WriteString(fmt.Sprintf("Project ID: %d\n", event.ProjectID))
		}

		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

func listGroupUsersHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	groupID := arguments["group_id"].(string)

	opt := &gitlab.ListGroupMembersOptions{
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
		},
	}

	members, _, err := gitlabClient().Groups.ListGroupMembers(groupID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to list group members: %v", err)
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Users in group %s:\n\n", groupID))

	for _, member := range members {
		result.WriteString(fmt.Sprintf("User: %s\n", member.Username))
		result.WriteString(fmt.Sprintf("Name: %s\n", member.Name))
		result.WriteString(fmt.Sprintf("ID: %d\n", member.ID))
		result.WriteString(fmt.Sprintf("State: %s\n", member.State))
		result.WriteString(fmt.Sprintf("Access Level: %s\n", getAccessLevelString(member.AccessLevel)))
		if member.ExpiresAt != nil {
			result.WriteString(fmt.Sprintf("Expires At: %s\n", member.ExpiresAt.String()))
		}
		result.WriteString("\n")
	}

	return mcp.NewToolResultText(result.String()), nil
}

// Helper function to convert access level to string
func getAccessLevelString(level gitlab.AccessLevelValue) string {
	switch level {
	case gitlab.GuestPermissions:
		return "Guest"
	case gitlab.ReporterPermissions:
		return "Reporter"
	case gitlab.DeveloperPermissions:
		return "Developer"
	case gitlab.MaintainerPermissions:
		return "Maintainer"
	case gitlab.OwnerPermission:
		return "Owner"
	default:
		return fmt.Sprintf("Unknown (%d)", level)
	}
}

func createMergeRequestHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	projectID := arguments["project_path"].(string)
	sourceBranch := arguments["source_branch"].(string)
	targetBranch := arguments["target_branch"].(string)
	title := arguments["title"].(string)

	opt := &gitlab.CreateMergeRequestOptions{
		Title:        gitlab.String(title),
		SourceBranch: gitlab.String(sourceBranch),
		TargetBranch: gitlab.String(targetBranch),
	}

	// Add description if provided
	if description, ok := arguments["description"]; ok {
		opt.Description = gitlab.String(description.(string))
	}

	mr, _, err := gitlabClient().MergeRequests.CreateMergeRequest(projectID, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %v", err)
	}

	result := strings.Builder{}
	result.WriteString("Merge Request created successfully!\n\n")
	result.WriteString(fmt.Sprintf("MR #%d: %s\n", mr.IID, mr.Title))
	result.WriteString(fmt.Sprintf("State: %s\n", mr.State))
	result.WriteString(fmt.Sprintf("Source Branch: %s\n", mr.SourceBranch))
	result.WriteString(fmt.Sprintf("Target Branch: %s\n", mr.TargetBranch))
	result.WriteString(fmt.Sprintf("Author: %s\n", mr.Author.Username))
	result.WriteString(fmt.Sprintf("Created: %s\n", mr.CreatedAt.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("URL: %s\n", mr.WebURL))

	if mr.Description != "" {
		result.WriteString("\nDescription:\n")
		result.WriteString(mr.Description)
	}

	return mcp.NewToolResultText(result.String()), nil
}

func cloneRepoHandler(arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	projectPath := arguments["project_path"].(string)
	ref := ""
	if value, ok := arguments["ref"]; ok {
		ref = value.(string)
	}

	localPath, err := repoCache.ensureRepo(projectPath, ref)
	if err != nil {
		return nil, err
	}

	// Get the actual commit SHA for the ref
	cmd := exec.Command("git", "-C", localPath, "rev-parse", ref)
	sha, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get commit SHA: %v", err)
	}

	result := strings.Builder{}
	result.WriteString("Repository cloned/updated successfully!\n")
	result.WriteString(fmt.Sprintf("Local path: %s\n", localPath))
	result.WriteString(fmt.Sprintf("Reference: %s\n", ref))
	result.WriteString(fmt.Sprintf("Commit SHA: %s", strings.TrimSpace(string(sha))))

	return mcp.NewToolResultText(result.String()), nil
}
