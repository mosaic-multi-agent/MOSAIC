package settings

// settingsTemplate is the documentation written at the head of a settings file
// that LogAnalyzer creates because it was absent. It consists only of comment
// and blank lines, so a file holding just the template loads as the defaults.
// Later writes keep it because they preserve the file's leading comment block.
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
#   vscode-ghcp: UNVERIFIED defaults; from agent frontmatter and system material, not real MOSAIC logs.
# Correct a list here when a live run shows a different tool name.
#
# Example
#
#   write_tools:
#     claude-code: [Write, Edit]
#   user_interaction_tools:
#     claude-code: [AskUserQuestion, "mcp__contact-user__*"]
#   detected_compaction:
#     drop_percent: 30

`
