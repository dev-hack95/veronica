# Veronica

## FEATURES

[ ] Format Code base in more managable and scalable way
     (status: REPORT.md — current structure supports ~5k-10k lines; 50k-60k requires the
      refactor program listed in §6: TUI decomposition, split `structs`, client interface
      + tests for harness/Update seams, CI/lint)

[x] Integrate MCP servers with custom harness. (design: assets/design/harness.md)
     (done: app/harness.go loop, tools exposed via GetOpenAITools, ExecuteToolCtx with
      per-tool timeouts, harness status table in assets/design/harness.md all implemented)
