# AIO-MCP Server

[![smithery badge](https://smithery.ai/badge/@athapong/aio-mcp)](https://smithery.ai/server/@athapong/aio-mcp)
A powerful Model Context Protocol (MCP) server implementation with integrations for GitLab, Jira, Confluence, YouTube, and more. This server provides AI-powered search capabilities and various utility tools for development workflows.

## Prerequisites

- Go 1.23.2 or higher
- Various API keys and tokens for the services you want to use

## Installation

### Installing via Smithery

To install AIO-MCP Server for Claude Desktop automatically via [Smithery](https://smithery.ai/server/@athapong/aio-mcp) (will guide you through interactive CLI setup):

```bash
npx -y @smithery/cli install @athapong/aio-mcp --client claude
```

*Note: Smithery will interactively prompt you for required configuration values and handle environment setup automatically*

### Installing via Go

To set the `go install` command to install into the Go bin path, you need to ensure your Go environment variables are correctly configured. Here's how to do it:

1. First, ensure you have a properly set `GOPATH` environment variable, which by default is `$HOME/go` on Unix-like systems or `%USERPROFILE%\go` on Windows.

2. The `go install` command places binaries in `$GOPATH/bin` by default. Make sure this directory is in your system's `PATH` environment variable.

Here's how to set this up on different operating systems:

### Linux/macOS:
```bash
# Add these to your ~/.bashrc, ~/.zshrc, or equivalent shell config file
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

After adding these lines, reload your shell configuration:
```bash
source ~/.bashrc  # or ~/.zshrc
```

### Windows (PowerShell):
```powershell
# Set environment variables
[Environment]::SetEnvironmentVariable("GOPATH", "$env:USERPROFILE\go", "User")
[Environment]::SetEnvironmentVariable("PATH", "$env:PATH;$env:USERPROFILE\go\bin", "User")
```

### Windows (Command Prompt):
```cmd
setx GOPATH "%USERPROFILE%\go"
setx PATH "%PATH%;%USERPROFILE%\go\bin"
```

After setting these variables, you can verify they're working correctly with:
```bash
go env GOPATH
echo $PATH  # On Unix/Linux/macOS
echo %PATH%  # On Windows CMD
$env:PATH  # On Windows PowerShell
```

Now when you run `go install`, the binaries will be installed to your `$GOPATH/bin` directory, which is in your PATH, so you can run them from anywhere.

Finally, install the server:
```bash
go install github.com/athapong/aio-mcp@latest
```

2. **Manual setup required** - Create a `.env` file with your configuration:
```env
ENABLE_TOOLS=
QDRANT_HOST=
ATLASSIAN_HOST=
ATLASSIAN_EMAIL=
GITLAB_HOST=
GITLAB_TOKEN=
BRAVE_API_KEY=
ATLASSIAN_TOKEN=
GOOGLE_AI_API_KEY=
PROXY_URL=
OPENAI_API_KEY=
OPENAI_EMBEDDING_MODEL=
DEEPSEEK_API_KEY=
QDRANT_PORT=
GOOGLE_TOKEN_FILE=
GOOGLE_CREDENTIALS_FILE=
QDRANT_API_KEY=
USE_OLLAMA_DEEPSEEK=
ENABLE_SSE=
SSE_ADDR=
SSE_BASE_PATH=
```

3. Config your claude's config:

```json{claude_desktop_config.json}
{
  "mcpServers": {
    "aio-mcp": {
      "command": "aio-mcp",
      "args": ["-env", "/path/to/.env", "-sse", "-sse-addr", ":8080", "-sse-base-path", "/mcp"],

    }
  }
}
```
### or override environment values as
```json
{
  "mcpServers": {
    "aio-mcp": {
      "command": "aio-mcp",
      "env": {
        "ENABLE_TOOLS": "",
        "OPENAI_BASE_URL": "",
        "GOOGLE_AI_API_KEY": "",
        "GITLAB_TOKEN": "",
        "GITLAB_HOST": "",
        "QDRANT_HOST": "",
        "QDRANT_API_KEY": "",

        "PROXY_URL": "",
        "OPENAI_API_KEY": "",
        "GOOGLE_TOKEN_FILE": "",
        "GOOGLE_CREDENTIALS_FILE": "",

        "ATLASSIAN_TOKEN": "",
        "BRAVE_API_KEY": "",
        "QDRANT_PORT": "",
        "ATLASSIAN_HOST": "",
        "ATLASSIAN_EMAIL": "",

        "USE_OPENROUTER": "", // "true" if you want to use openrouter for AI to help with reasoning on `tool_use_plan`, default is false
        "DEEPSEEK_API_KEY": "", // specify the deepseek api key if you want to use deepseek for AI to help with reasoning on `tool_use_plan`
        "OPENROUTER_API_KEY": "", // specify the openrouter api key if you want to use openrouter for AI to help with reasoning on `tool_use_plan`
        "DEEPSEEK_API_BASE": "", // specify the deepseek api key if you want to use deepseek for AI to help with reasoning on `tool_use_plan`
        "USE_OLLAMA_DEEPSEEK": "", // "true" if you want to use deepseek with local ollama, default is false
        "OLLAMA_URL": "" // default with http://localhost:11434
      }
    }
  }
}
```

## Server Modes

AIO-MCP Server supports two modes of operation:

1. **Stdio Mode (Default)**: The server communicates via standard input/output, which is the default mode used by Claude Desktop and other MCP clients.

2. **SSE (Server-Sent Events) Mode**: The server runs as an HTTP server that supports Server-Sent Events for real-time communication. This is useful for web-based clients or when you need to access the MCP server over a network.

### Enabling SSE Mode

You can enable SSE mode in one of two ways:

1. **Command-line flags**:
   ```bash
   aio-mcp -sse -sse-addr ":8080" -sse-base-path "/mcp"
   ```

2. **Environment variables** (in your `.env` file):
   ```
   ENABLE_SSE=true
   SSE_ADDR=:8080
   SSE_BASE_PATH=/mcp
   ```

When SSE mode is enabled, the server will start an HTTP server that listens on the specified address. The server provides two endpoints:
- SSE endpoint: `{SSE_BASE_PATH}/sse` (default: `/mcp/sse`)
- Message endpoint: `{SSE_BASE_PATH}/message` (default: `/mcp/message`)

Clients can connect to the SSE endpoint to receive server events and send messages to the message endpoint.

## Enable Tools

There is a hidden variable `ENABLE_TOOLS` in the environment variable. It is a comma separated list of tools group to enable. If not set, all tools will be enabled. Leave it empty to enable all tools.


Here is the list of tools group:

- `gemini`: Gemini-powered search
- `fetch`: Fetch tools
- `brave_search`: Brave Search tools
- `google_maps`: Google Maps tools
- `confluence`: Confluence tools
- `youtube`: YouTube tools
- `jira`: Jira tools
- `gitlab`: GitLab tools
- `script`: Script tools
- `rag`: RAG tools
- `deepseek`: Deepseek AI tools, including reasoning and advanced search if 'USE_OLLAMA_DEEPSEEK' is set to true, default ollama endpoint is http://localhost:11434 with model deepseek-r1:8b

## Available Tools

### calendar_create_event

Create a new event in Google Calendar

Arguments:

- `summary` (String) (Required): Title of the event
- `description` (String): Description of the event
- `start_time` (String) (Required): Start time of the event in RFC3339 format (e.g., 2023-12-25T09:00:00Z)
- `end_time` (String) (Required): End time of the event in RFC3339 format
- `attendees` (String): Comma-separated list of attendee email addresses

### calendar_list_events

List upcoming events in Google Calendar

Arguments:

- `time_min` (String): Start time for the search in RFC3339 format (default: now)
- `time_max` (String): End time for the search in RFC3339 format (default: 1 week from now)
- `max_results` (Number): Maximum number of events to return (default: 10)

### calendar_update_event

Update an existing event in Google Calendar

Arguments:

- `event_id` (String) (Required): ID of the event to update
- `summary` (String): New title of the event
- `description` (String): New description of the event
- `start_time` (String): New start time of the event in RFC3339 format
- `end_time` (String): New end time of the event in RFC3339 format
- `attendees` (String): Comma-separated list of new attendee email addresses

### calendar_respond_to_event

Respond to an event invitation in Google Calendar

Arguments:

- `event_id` (String) (Required): ID of the event to respond to
- `response` (String) (Required): Your response (accepted, declined, or tentative)

### confluence_search

Search Confluence

Arguments:

- `query` (String) (Required): Atlassian Confluence Query Language (CQL)

### confluence_get_page

Get Confluence page content

Arguments:

- `page_id` (String) (Required): Confluence page ID

### confluence_create_page

Create a new Confluence page

Arguments:

- `space_key` (String) (Required): The key of the space where the page will be created
- `title` (String) (Required): Title of the page
- `content` (String) (Required): Content of the page in storage format (XHTML)
- `parent_id` (String): ID of the parent page (optional)

### confluence_update_page

Update an existing Confluence page

Arguments:

- `page_id` (String) (Required): ID of the page to update
- `title` (String): New title of the page (optional)
- `content` (String): New content of the page in storage format (XHTML)
- `version_number` (String): Version number for optimistic locking (optional)

### confluence_compare_versions

Compare two versions of a Confluence page

Arguments:

- `page_id` (String) (Required): Confluence page ID
- `source_version` (String) (Required): Source version number
- `target_version` (String) (Required): Target version number

### deepseek_reasoning

advanced reasoning engine using Deepseek's AI capabilities for multi-step problem solving, critical analysis, and strategic decision support

Arguments:

- `question` (String) (Required): The structured query or problem statement requiring deep analysis and reasoning
- `context` (String) (Required): Defines the operational context and purpose of the query within the MCP ecosystem
- `knowledge` (String): Provides relevant chat history, knowledge base entries, and structured data context for MCP-aware reasoning

### get_web_content

Fetches content from a given HTTP/HTTPS URL. This tool allows you to retrieve text content from web pages, APIs, or any accessible HTTP endpoints. Returns the raw content as text.

Arguments:

- `url` (String) (Required): The complete HTTP/HTTPS URL to fetch content from (e.g., https://example.com)

### gchat_list_spaces

List all available Google Chat spaces/rooms

### gchat_send_message

Send a message to a Google Chat space or direct message

Arguments:

- `space_name` (String) (Required): Name of the space to send the message to
- `message` (String) (Required): Text message to send

### ai_web_search

search the web by using Google AI Search. Best tool to update realtime information

Arguments:

- `question` (String) (Required): The question to ask. Should be a question
- `context` (String) (Required): Context/purpose of the question, helps Gemini to understand the question better

### gitlab_list_projects

List GitLab projects

Arguments:

- `group_id` (String) (Required): gitlab group ID
- `search` (String): Multiple terms can be provided, separated by an escaped space, either + or %20, and will be ANDed together. Example: one+two will match substrings one and two (in any order).

### gitlab_get_project

Get GitLab project details

Arguments:

- `project_path` (String) (Required): Project/repo path

### gitlab_list_mrs

List merge requests

Arguments:

- `project_path` (String) (Required): Project/repo path
- `state` (String) (Default: all): MR state (opened/closed/merged)

### gitlab_get_mr_details

Get merge request details

Arguments:

- `project_path` (String) (Required): Project/repo path
- `mr_iid` (String) (Required): Merge request IID

### gitlab_create_MR_note

Create a note on a merge request

Arguments:

- `project_path` (String) (Required): Project/repo path
- `mr_iid` (String) (Required): Merge request IID
- `comment` (String) (Required): Comment text

### gitlab_get_file_content

Get file content from a GitLab repository

Arguments:

- `project_path` (String) (Required): Project/repo path
- `file_path` (String) (Required): Path to the file in the repository
- `ref` (String) (Required): Branch name, tag, or commit SHA

### gitlab_list_pipelines

List pipelines for a GitLab project

Arguments:

- `project_path` (String) (Required): Project/repo path
- `status` (String) (Default: all): Pipeline status (running/pending/success/failed/canceled/skipped/all)

### gitlab_list_commits

List commits in a GitLab project within a date range

Arguments:

- `project_path` (String) (Required): Project/repo path
- `since` (String) (Required): Start date (YYYY-MM-DD)
- `until` (String): End date (YYYY-MM-DD). If not provided, defaults to current date
- `ref` (String) (Required): Branch name, tag, or commit SHA

### gitlab_get_commit_details

Get details of a commit

Arguments:

- `project_path` (String) (Required): Project/repo path
- `commit_sha` (String) (Required): Commit SHA

### gitlab_list_user_events

List GitLab user events within a date range

Arguments:

- `username` (String) (Required): GitLab username
- `since` (String) (Required): Start date (YYYY-MM-DD)
- `until` (String): End date (YYYY-MM-DD). If not provided, defaults to current date

### gitlab_list_group_users

List all users in a GitLab group

Arguments:

- `group_id` (String) (Required): GitLab group ID

### gitlab_create_mr

Create a new merge request

Arguments:

- `project_path` (String) (Required): Project/repo path
- `source_branch` (String) (Required): Source branch name
- `target_branch` (String) (Required): Target branch name
- `title` (String) (Required): Merge request title
- `description` (String): Merge request description

### gitlab_clone_repo

Clone or update a GitLab repository locally

Arguments:

- `project_path` (String) (Required): Project/repo path
- `ref` (String): Branch name or tag (optional, defaults to project's default branch)

### gmail_search

Search emails in Gmail using Gmail's search syntax

Arguments:

- `query` (String) (Required): Gmail search query. Follow Gmail's search syntax

### gmail_move_to_spam

Move specific emails to spam folder in Gmail by message IDs

Arguments:

- `message_ids` (String) (Required): Comma-separated list of message IDs to move to spam

### gmail_create_filter

Create a Gmail filter with specified criteria and actions

Arguments:

- `from` (String): Filter emails from this sender
- `to` (String): Filter emails to this recipient
- `subject` (String): Filter emails with this subject
- `query` (String): Additional search query criteria
- `add_label` (Boolean): Add label to matching messages
- `label_name` (String): Name of the label to add (required if add_label is true)
- `mark_important` (Boolean): Mark matching messages as important
- `mark_read` (Boolean): Mark matching messages as read
- `archive` (Boolean): Archive matching messages

### gmail_list_filters

List all Gmail filters in the account

### gmail_list_labels

List all Gmail labels in the account

### gmail_delete_filter

Delete a Gmail filter by its ID

Arguments:

- `filter_id` (String) (Required): The ID of the filter to delete

### gmail_delete_label

Delete a Gmail label by its ID

Arguments:

- `label_id` (String) (Required): The ID of the label to delete

### jira_get_issue

Retrieve detailed information about a specific Jira issue including its status, assignee, description, subtasks, and available transitions

Arguments:

- `issue_key` (String) (Required): The unique identifier of the Jira issue (e.g., KP-2, PROJ-123)

### jira_search_issue

Search for Jira issues using JQL (Jira Query Language). Returns key details like summary, status, assignee, and priority for matching issues

Arguments:

- `jql` (String) (Required): JQL query string (e.g., 'project = KP AND status = \"In Progress\"')

### jira_list_sprints

List all active and future sprints for a specific Jira board, including sprint IDs, names, states, and dates

Arguments:

- `board_id` (String) (Required): Numeric ID of the Jira board (can be found in board URL)

### jira_create_issue

Create a new Jira issue with specified details. Returns the created issue's key, ID, and URL

Arguments:

- `project_key` (String) (Required): Project identifier where the issue will be created (e.g., KP, PROJ)
- `summary` (String) (Required): Brief title or headline of the issue
- `description` (String) (Required): Detailed explanation of the issue
- `issue_type` (String) (Required): Type of issue to create (common types: Bug, Task, Story, Epic)

### jira_update_issue

Modify an existing Jira issue's details. Supports partial updates - only specified fields will be changed

Arguments:

- `issue_key` (String) (Required): The unique identifier of the issue to update (e.g., KP-2)
- `summary` (String): New title for the issue (optional)
- `description` (String): New description for the issue (optional)

### jira_list_statuses

Retrieve all available issue status IDs and their names for a specific Jira project

Arguments:

- `project_key` (String) (Required): Project identifier (e.g., KP, PROJ)

### jira_transition_issue

Transition an issue through its workflow using a valid transition ID. Get available transitions from jira_get_issue

Arguments:

- `issue_key` (String) (Required): The issue to transition (e.g., KP-123)
- `transition_id` (String) (Required): Transition ID from available transitions list
- `comment` (String): Optional comment to add with transition

### RAG_memory_index_content

Index a content into memory, can be inserted or updated

Arguments:

- `collection` (String) (Required): Memory collection name
- `filePath` (String) (Required): content file path
- `payload` (String) (Required): Plain text payload
- `model` (String): Embedding model to use (default: text-embedding-3-large)

### RAG_memory_index_file

Index a local file into memory

Arguments:

- `collection` (String) (Required): Memory collection name
- `filePath` (String) (Required): Path to the local file to be indexed

### RAG_memory_create_collection

Create a new vector collection in memory

Arguments:

- `collection` (String) (Required): Memory collection name
- `model` (String): Embedding model to use (default: text-embedding-3-large)

### RAG_memory_delete_collection

Delete a vector collection in memory

Arguments:

- `collection` (String) (Required): Memory collection name

### RAG_memory_list_collections

List all vector collections in memory

### RAG_memory_search

Search for memory in a collection based on a query

Arguments:

- `collection` (String) (Required): Memory collection name
- `query` (String) (Required): search query, should be a keyword
- `model` (String): Embedding model to use (default: text-embedding-3-large)

### RAG_memory_delete_index_by_filepath

Delete a vector index by filePath

Arguments:

- `collection` (String) (Required): Memory collection name
- `filePath` (String) (Required): Path to the local file to be deleted

### execute_comand_line_script

Safely execute command line scripts on the user's system with security restrictions. Features sandboxed execution, timeout protection, and output capture. Supports cross-platform scripting with automatic environment detection.

Arguments:

- `content` (String) (Required):
- `interpreter` (String) (Default: /bin/sh): Path to interpreter binary (e.g. /bin/sh, /bin/bash, /usr/bin/python, cmd.exe). Validated against allowed list for security
- `working_dir` (String): Execution directory path (default: user home). Validated to prevent unauthorized access to system locations

### web_search

Search the web using Brave Search API

Arguments:

- `query` (String) (Required): Query to search for (max 400 chars, 50 words)
- `count` (Number) (Default: 5): Number of results (1-20, default 5)
- `country` (String) (Default: ALL): Country code

### sequentialthinking

`A detailed tool for dynamic and reflective problem-solving through thoughts.
This tool helps analyze problems through a flexible thinking process that can adapt and evolve.
Each thought can build on, question, or revise previous insights as understanding deepens.

When to use this tool:
- Breaking down complex problems into steps
- Planning and design with room for revision
- Analysis that might need course correction
- Problems where the full scope might not be clear initially
- Problems that require a multi-step solution
- Tasks that need to maintain context over multiple steps
- Situations where irrelevant information needs to be filtered out

Key features:
- You can adjust total_thoughts up or down as you progress
- You can question or revise previous thoughts
- You can add more thoughts even after reaching what seemed like the end
- You can express uncertainty and explore alternative approaches
- Not every thought needs to build linearly - you can branch or backtrack
- Generates a solution hypothesis
- Verifies the hypothesis based on the Chain of Thought steps
- Repeats the process until satisfied
- Provides a correct answer

Parameters explained:
- thought: Your current thinking step, which can include:
* Regular analytical steps
* Revisions of previous thoughts
* Questions about previous decisions
* Realizations about needing more analysis
* Changes in approach
* Hypothesis generation
* Hypothesis verification
- next_thought_needed: True if you need more thinking, even if at what seemed like the end
- thought_number: Current number in sequence (can go beyond initial total if needed)
- total_thoughts: Current estimate of thoughts needed (can be adjusted up/down)
- is_revision: A boolean indicating if this thought revises previous thinking
- revises_thought: If is_revision is true, which thought number is being reconsidered
- branch_from_thought: If branching, which thought number is the branching point
- branch_id: Identifier for the current branch (if any)
- needs_more_thoughts: If reaching end but realizing more thoughts needed

You should:
1. Start with an initial estimate of needed thoughts, but be ready to adjust
2. Feel free to question or revise previous thoughts
3. Don't hesitate to add more thoughts if needed, even at the "end"
4. Express uncertainty when present
5. Mark thoughts that revise previous thinking or branch into new paths
6. Ignore information that is irrelevant to the current step
7. Generate a solution hypothesis when appropriate
8. Verify the hypothesis based on the Chain of Thought steps
9. Repeat the process until satisfied with the solution
10. Provide a single, ideally correct answer as the final output
11. Only set next_thought_needed to false when truly done and a satisfactory answer is reached`

Arguments:

- `thought` (String) (Required): Your current thinking step
- `nextThoughtNeeded` (Boolean) (Required): Whether another thought step is needed
- `thoughtNumber` (Number) (Required): Current thought number
- `totalThoughts` (Number) (Required): Estimated total thoughts needed
- `isRevision` (Boolean): Whether this revises previous thinking
- `revisesThought` (Number): Which thought is being reconsidered
- `branchFromThought` (Number): Branching point thought number
- `branchId` (String): Branch identifier
- `needsMoreThoughts` (Boolean): If more thoughts are needed
- `result` (String): Final result or conclusion from this thought
- `summary` (String): Brief summary of the thought's key points

### sequentialthinking_history

Retrieve the thought history for the current thinking process

Arguments:

- `branchId` (String): Optional branch ID to get history for

### tool_manager

Manage MCP tools - enable or disable tools

Arguments:

- `action` (String) (Required): Action to perform: list, enable, disable
- `tool_name` (String): Tool name to enable/disable

### tool_use_plan

Create a plan using available tools to solve the request

Arguments:

- `request` (String) (Required): Request to plan for
- `context` (String) (Required): Context related to the request

### youtube_transcript

Get YouTube video transcript

Arguments:

- `video_id` (String) (Required): YouTube video ID

### youtube_update_video

Update a video's title and description on YouTube

Arguments:

- `video_id` (String) (Required): ID of the video to update
- `title` (String) (Required): New title of the video
- `description` (String) (Required): New description of the video
- `keywords` (String) (Required): Comma-separated list of keywords for the video
- `category` (String) (Required): Category ID for the video. See https://developers.google.com/youtube/v3/docs/videoCategories/list for more information.

### youtube_get_video_details

Get details (title, description, ...) for a specific video

Arguments:

- `video_id` (String) (Required): ID of the video

### youtube_list_videos

List YouTube videos managed by the user

Arguments:

- `channel_id` (String) (Required): ID of the channel to list videos for
- `max_results` (Number) (Required): Maximum number of videos to return
