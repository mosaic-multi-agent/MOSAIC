package settings

// settingsTemplate is the documentation written at the head of a settings file
// that LogAnalyzer creates because it was absent, and the content of a new
// global file. It consists only of comment and blank lines, so a file holding
// just the template loads as the defaults. Later writes keep it because they
// preserve the file's leading comment block.
const settingsTemplate = `# MosaicLogAnalyzer analysis settings
#
# Every key is optional. An absent key keeps the built-in default.
#
# write_tools
#   Tool names that write file content, per harness.
# user_interaction_tools
#   Tool names that ask the user a question, per harness.
# detected_compaction.drop_percent
#   A fall of the context size by at least this percentage (1 to 99)
#   between two consecutive calls is a detected compaction. Default: 30.
#
# default_cache_write_tier
#   Per harness, the cache-write tier (5m or 1h) that prices cache writes a
#   request reports without a 5m/1h split. Default: 5m. Records that carry
#   the split are priced as reported.
#
# A harness key is the harness name as logged (claude-code, ghcp-cli,
# opencode, vscode-ghcp). A harness list written here REPLACES that harness's
# default list; an empty list ([]) means no tools. A harness you do not list
# keeps its default list.
#
# A tool name is exact, or a pattern where '*' matches any run of characters
# (including none), for example "mcp__contact-user__*". Every other character
# is literal. Matching is case-sensitive over the whole name. Quote patterns.
#
# Built-in defaults
#   write_tools
#     claude-code:  Write, Edit, MultiEdit, NotebookEdit
#     ghcp-cli:     create, edit
#     opencode:     apply_patch
#     vscode-ghcp:  create_file, replace_string_in_file,
#                   multi_replace_string_in_file, insert_edit_into_file,
#                   apply_patch, edit_notebook_file, edit/createFile,
#                   edit/editFiles
#   user_interaction_tools
#     claude-code:  AskUserQuestion
#     ghcp-cli:     ask_user
#     opencode:     question
#     vscode-ghcp:  vscode/askQuestions, askQuestions, vscode_askQuestions
#
# Verification status of the defaults
#   claude-code: verified (Write, Edit and AskUserQuestion seen in real logs).
#   ghcp-cli: UNVERIFIED defaults; from captured system prompts, not real MOSAIC logs.
#   opencode: UNVERIFIED defaults; from captured system prompts, not real MOSAIC logs.
#   vscode-ghcp: partly verified (create_file, replace_string_in_file,
#     multi_replace_string_in_file and vscode_askQuestions seen in real logs).
#     The other vscode-ghcp names are UNVERIFIED; they come from agent
#     frontmatter and system material, not real MOSAIC logs.
# Correct a list here when a live run shows a different tool name.
#
# MCP tool names in VS Code (vscode-ghcp)
#   VS Code names an MCP tool "mcp_<server>_<tool>" and SHORTENS the server
#   name, so a long server name is cut off: "azure-devops-..." becomes
#   "mcp_azure-devops-_<tool>". When two registered servers end up with the
#   same shortened prefix (for example the same server in both user settings
#   and the workspace .vscode/mcp.json), VS Code replaces the last character
#   with a number, so "mcp_userinterface_userFeedback" can also appear as
#   "mcp_userinterfac2_userFeedback". Match such tools with a pattern, for
#   example "mcp_userinterfac*_userFeedback", never an exact name. List the
#   vscode-ghcp defaults as well, because a harness list replaces the defaults.
#
# Example
#
#   write_tools:
#     claude-code: [Write, Edit]
#   user_interaction_tools:
#     claude-code: [AskUserQuestion, "mcp__contact-user__*"]
#   default_cache_write_tier:
#     claude-code: 1h
#   detected_compaction:
#     drop_percent: 30

`
