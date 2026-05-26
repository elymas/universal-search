# Universal Search -- Claude Code Plugin

Team-scale research meta-agent for Claude Code. Search across web, social, academic, and Korean sources with cited synthesis and deep-research reports.

## Quick Start

### Prerequisites

1. **Install the `usearch-mcp` binary**

   ```bash
   # From source (requires Go 1.23+)
   go install github.com/elymas/universal-search/cmd/usearch-mcp@latest

   # Or build from this repo
   cd /path/to/universal-search && make build
   ```

   Verify the binary is on your PATH:

   ```bash
   usearch-mcp --version
   ```

2. **Configure Universal Search**

   ```bash
   usearch config init
   ```

   This interactive wizard sets your search endpoints, default sources, and authentication. Configuration is stored at `~/.config/usearch/config.toml`.

3. **Install the plugin**

   From a Claude Code session:

   ```
   /plugin install --plugin-dir /path/to/this/directory
   ```

   Or, once listed in the community marketplace:

   ```
   /plugin marketplace add anthropics/claude-plugins-community
   /plugin install @claude-community/usearch
   ```

### Verify

Open a Claude Code conversation and ask a research question:

> "What are the latest developments in AI regulation with citations?"

Claude should invoke the `search` tool from the Universal Search MCP server and return a cited answer.

## Compatibility

| Plugin Version | Minimum `usearch-mcp` Version | Notes                                |
| -------------- | ----------------------------- | ------------------------------------ |
| 0.1.0          | 0.1.0-dev                     | Initial plugin, stdio transport only |

Check your server version:

```bash
usearch-mcp --version
```

## What It Does

The plugin exposes four MCP tools that Claude can invoke automatically:

| Tool            | When Claude Uses It                                                       |
| --------------- | ------------------------------------------------------------------------- |
| `search`        | User asks a research question needing citations or multi-source synthesis |
| `deep_research` | User requests a comprehensive, long-form report                           |
| `list_sources`  | User asks what search sources are available                               |
| `get_citation`  | User wants details on a specific cited source                             |

Korean-language queries are automatically routed to Korean sources (Naver, Daum, Korean RSS) -- no configuration needed.

## Team Deployment (HTTP Transport)

For team-shared deployments with a remote `usearch-mcp` HTTP server, replace `.mcp.json` with:

```json
{
  "mcpServers": {
    "usearch": {
      "type": "http",
      "url": "https://search.yourteam.example.com/mcp",
      "headers": {
        "Authorization": "Bearer ${USEARCH_TOKEN}"
      }
    }
  }
}
```

Set the `USEARCH_TOKEN` environment variable to your JWT:

```bash
export USEARCH_TOKEN="eyJ..."
```

**Warning:** Never commit `.mcp.json` containing literal tokens to version control. Always use the `${USEARCH_TOKEN}` environment variable substitution.

## Troubleshooting

### "command not found: usearch-mcp"

The `usearch-mcp` binary is not on your PATH. Install it (see Quick Start step 1) and verify with `usearch-mcp --version`.

### Config file missing or invalid

Run `usearch config init` to create or repair the configuration file at `~/.config/usearch/config.toml`.

### Daily limit reached

Deep-research reports have a daily quota. The error message includes the reset time. Ask your team admin about quota limits.

### MCP server shows "disconnected" in Claude Code

Check that `usearch-mcp` runs successfully outside of Claude Code:

```bash
echo '{"jsonrpc":"2.0","method":"initialize","id":1,"params":{}}' | usearch-mcp --transport stdio
```

If this fails, the issue is in the server configuration, not the plugin.

## Other MCP Hosts

This plugin is designed for **Claude Code** and **Claude Desktop** only.

For other MCP-capable hosts (Gemini CLI, Codex CLI, etc.), install `usearch-mcp` directly and configure your host's MCP settings. See the [Universal Search MCP documentation](https://github.com/elymas/universal-search/tree/main/docs) for host-specific instructions.

## License

Apache-2.0. See [LICENSE](./LICENSE).
