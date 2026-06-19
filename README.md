# CEL Skills

Collection of skills and associated Model Context Protocol (MCP) server for working with Google CEL (Common Expression Language).

---

## ♊ Gemini & Antigravity Configuration

Gemini CLI and Antigravity auto-discover workflows (skills) placed under the `.agents/skills` directory of the workspace.

### 1. Build the MCP Server
Build the Go binary in the root of the repository:
```bash
go build -o bin/cel-mcp cmd/mcp/main.go
```

### 2. Configure MCP Server in Gemini
To register the MCP server, add it to your local Gemini/Jetski configuration file (`~/.gemini/jetski/mcp_config.json`):
```json
{
  "mcpServers": {
    "cel-mcp": {
      "command": "/absolute/path/to/cel-expr/skills/bin/cel-mcp",
      "args": [],
      "env": {}
    }
  }
}
```

### 3. Usage
Once configured, you can invoke the skills by their name from the Gemini CLI or Antigravity prompt. For example:
```
/cel:cel-authoring create a policy that checks if a user's age is over 18.
```
The agent will follow the workflow in `.agents/skills/cel-authoring/SKILL.md`, automatically calling `cel_create_environment`, `cel_generate_prompt`, and `cel_compile` as needed.

---

## 🔌 Claude Desktop Configuration

Claude Desktop supports native integration with MCP servers through standard configuration.

### 1. Configure MCP Server in Claude
Add the server configuration to your Claude Desktop config file:
* **macOS:** `~/Library/Application Support/Claude/claude_desktop_config.json`
* **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

Add the following to your `"mcpServers"` object (replacing with your repository's absolute path):
```json
{
  "mcpServers": {
    "cel-mcp": {
      "command": "/absolute/path/to/cel-expr/skills/bin/cel-mcp",
      "args": [],
      "env": {}
    }
  }
}
```

### 2. Usage
Restart Claude Desktop. The `cel-mcp` tools will now be available in the tool tray and can be queried or run directly by Claude.

---

## 🚀 Cursor Configuration

Cursor supports connecting to any local MCP server via a command transport.

### 1. Add MCP Server in Cursor
1. Open Cursor and navigate to **Settings** (Gear icon in top right) ➡️ **Features** ➡️ **MCP**.
2. Click **+ Add New MCP Server**.
3. Fill out the configuration:
   * **Name:** `cel-mcp`
   * **Type:** `command`
   * **Command:** `/absolute/path/to/cel-expr/skills/bin/cel-mcp`
4. Click **Save**.

### 2. Usage
The tools will immediately display green/active in your Cursor settings and can be leveraged by Cursor Chat or Composer via natural language.

