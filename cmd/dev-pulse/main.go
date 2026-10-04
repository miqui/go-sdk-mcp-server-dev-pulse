// Command dev-pulse is a read-only MCP server exposing "workspace pulse"
// tools over a local git workspace root: repo_status, ci_status, pr_queue,
// and stale_branches. It communicates over stdio; all logging goes to
// stderr so stdout stays reserved for the MCP JSON-RPC channel.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/execx"
	"github.com/miqui/go-sdk-mcp-server-dev-pulse/internal/tools"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "dev-pulse",
		Version: "v0.1.0",
	}, nil)

	tools.RegisterAll(server, tools.Deps{Runner: execx.OSRunner{}})

	logger.Info("dev-pulse starting", "transport", "stdio")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}
