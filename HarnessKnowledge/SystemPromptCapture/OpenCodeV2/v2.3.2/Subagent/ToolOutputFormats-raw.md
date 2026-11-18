# Tool Output Formats — Raw Captures

| Field | Value |
|-------|-------|
| **Harness** | OpenCode V2 |
| **Harness version** | v2.3.2 |
| **Context** | Subagent session |
| **Captured** | 2026-11-18 |
| **Formatting** | XML tags use bracket notation: `<tag>` -> `[tag]` to avoid rendering conflicts |

---

## read — full file

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt"})`

--- RAW OUTPUT START ---
Read file HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt, lines 1-20
1: Line 1: Greetings, Universe! 🚀
2: Line 2: This file has "double quotes" and 'single quotes' for validation.
3: Line 3:     Four-space indentation here.
4: Line 4:	Tab-indented content here.
5: Line 5: Special characters: [chevrons] & ampersand | pipe \ backslash
6: Line 6: Unicode: resume, naive, 中文测试, Кириллица, עברית
7: Line 7: Blank line follows:
8: 
9: Line 9: Resumed after blank line.
10: Line 10: Path-like content: /opt/tools/bin/run --mode=capture
11: Line 11: Code-like content: const calc = (x) => { return x > 0 && x < 100; }
12: Line 12: Regex-like content: ^begin.*center[a-z]+finish$
13: Line 13: XML-like content: [node role="test"]nested value[/node]
14: Line 14: JSON-like content: {"name": "probe", "count": 77, "items": [4, 5, 6]}
15: --- EDIT TARGET START ---
16: Line 16: Swap this line with different content.
17: --- EDIT TARGET END ---
18: Line 18: This line must stay intact after edit tests.
19: Line 19: Grep target: DISTINCT_MATCH_TOKEN_ABC
20: Line 20: Final line with trailing newline.
--- RAW OUTPUT END ---

## read — partial file

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt","offset":3,"limit":3})`

--- RAW OUTPUT START ---
Read file HarnessKnowledge/SystemPromptCapture/TestArtifacts/varied-content-test.txt, lines 3-5
3: Line 3:     Four-space indentation here.
4: Line 4:	Tab-indented content here.
5: Line 5: Special characters: [chevrons] & ampersand | pipe \ backslash
[Output truncated. Continue reading with offset: 6]
--- RAW OUTPUT END ---

## read — directory

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
Read directory HarnessKnowledge/SystemPromptCapture/TestArtifacts, entries 1-2
large-file-3000-lines.txt
varied-content-test.txt
--- RAW OUTPUT END ---

## read — missing file

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/does-not-exist.txt"})`

--- RAW OUTPUT START ---
{"error":{"type":"tool.execution","message":"File not found: HarnessKnowledge/SystemPromptCapture/TestArtifacts/does-not-exist.txt"},"content":[]}
--- RAW OUTPUT END ---

## read — large file default limit

**Call:** `functions.read({"path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts/large-file-3000-lines.txt"})`

--- RAW OUTPUT START ---
[NOT REPRODUCIBLE FROM CONTEXT: 1422 line output exceeds practical verbatim transcription size; the visible output begins "Read file HarnessKnowledge/SystemPromptCapture/TestArtifacts/large-file-3000-lines.txt, lines 1-1422" and ends "[Output truncated. Continue reading with offset: 1423]"]
--- RAW OUTPUT END ---

## glob — matches

**Call:** `functions.glob({"pattern":"*.txt","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\large-file-3000-lines.txt
--- RAW OUTPUT END ---

## glob — no matches

**Call:** `functions.glob({"pattern":"missing-*.xyz","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
No files found
--- RAW OUTPUT END ---

## grep — matches

**Call:** `functions.grep({"pattern":"EDIT TARGET","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
Found 2 matches
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:
  Line 15: --- EDIT TARGET START ---

  Line 17: --- EDIT TARGET END ---
--- RAW OUTPUT END ---

## grep — no matches

**Call:** `functions.grep({"pattern":"unfindable-xyz-9132","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts"})`

--- RAW OUTPUT START ---
No matches found
--- RAW OUTPUT END ---

## grep — include filter

**Call:** `functions.grep({"pattern":"EDIT TARGET","path":"HarnessKnowledge/SystemPromptCapture/TestArtifacts","include":"*.txt"})`

--- RAW OUTPUT START ---
Found 2 matches
C:\AI\MOSAIC\MOSAIC\HarnessKnowledge\SystemPromptCapture\TestArtifacts\varied-content-test.txt:
  Line 15: --- EDIT TARGET START ---

  Line 17: --- EDIT TARGET END ---
--- RAW OUTPUT END ---

## patch — successful edit of temporary artifact copy

**Call:** `functions.patch("*** Begin Patch\n*** Update File: HarnessKnowledge/SystemPromptCapture/TestArtifacts/capture-edit-temp.txt\n@@\n-Line 16: Swap this line with different content.\n+Line 16: Replaced for the capture test.\n*** End Patch")`

--- RAW OUTPUT START ---
Success. Updated the following files:
M HarnessKnowledge/SystemPromptCapture/TestArtifacts/capture-edit-temp.txt
--- RAW OUTPUT END ---

## patch — missing old text

**Call:** `functions.patch("*** Begin Patch\n*** Update File: HarnessKnowledge/SystemPromptCapture/TestArtifacts/capture-edit-temp.txt\n@@\n-Line 16: No such string in this file.\n+Line 16: This edit should fail.\n*** End Patch")`

--- RAW OUTPUT START ---
{"error":{"type":"tool.execution","message":"patch verification failed: Failed to find expected lines in HarnessKnowledge/SystemPromptCapture/TestArtifacts/capture-edit-temp.txt:\nLine 16: No such string in this file."},"content":[]}
--- RAW OUTPUT END ---

## patch — successful write

**Call:** `functions.patch("*** Begin Patch\n*** Add File: HarnessKnowledge/SystemPromptCapture/TestArtifacts/capture-write-temp.txt\n+write test success\n*** End Patch")`

--- RAW OUTPUT START ---
Success. Updated the following files:
A HarnessKnowledge/SystemPromptCapture/TestArtifacts/capture-write-temp.txt
--- RAW OUTPUT END ---

## shell — success

**Call:** `functions.shell({"command":"Write-Output 'capture success'"})`

--- RAW OUTPUT START ---
capture success
--- RAW OUTPUT END ---

## shell — nonzero exit

**Call:** `functions.shell({"command":"exit 7"})`

--- RAW OUTPUT START ---
(no output)Exit code: 7
--- RAW OUTPUT END ---

## shell — long output

**Call:** `functions.shell({"command":"1..5000 | ForEach-Object { 'long-output-row-' + $_ }"})`

--- RAW OUTPUT START ---
[NOT REPRODUCIBLE FROM CONTEXT: 2000 displayed lines of shell output cannot be transcribed verbatim; output starts "long-output-row-3001", ends "long-output-row-5000" and appends "[showing lines 3001-5000 of 5000; full output saved to C:\Users\tgurt\.local\share\opencode\shell\b27e9ef462535a30ae764504d83aa0d9670e945f\sh_1f039768a001u0E500G5vT5TVo.out]"]
--- RAW OUTPUT END ---

## question — test response

**Call:** `functions.question({"questions":[{"question":"Capture test: choose an option so I can record this tool's output format.","header":"Capture test","options":[{"label":"Option A (Recommended)","description":"Select the first test option."},{"label":"Option B","description":"Select the second test option."}]}]})`

--- RAW OUTPUT START ---
User has answered your questions: "Capture test: choose an option so I can record this tool's output format."="Option A (Recommended)". You can now continue with the user's answers in mind.
--- RAW OUTPUT END ---

## subagent — Not captured: running in subagent tier; test spawn reserved for primary agent

## opencode_worktree_remove — Not captured: no disposable worktree exists and removing a user's worktree would be destructive

## skill — Not captured: no task-relevant skill ID available

## multi_tool_use.parallel — Not captured separately: wrapper used for the independent read, glob, grep and shell cases above; outputs arrived as separate tool results
