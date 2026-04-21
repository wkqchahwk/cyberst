# SSE MCP Test Server

This test server validates the SSE external MCP workflow.

## Usage

1. Start the server with `go run main.go`.
2. Add an external MCP config that points to `http://127.0.0.1:8082/sse`.
3. Use the sample tools `test_echo` and `test_add` to confirm request and response handling.
