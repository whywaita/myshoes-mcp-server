# myshoes-mcp-server

The myshoes MCP Server is [Model Context Protocol (MCP)](https://modelcontextprotocol.io/introduction) compliant server for the [whywaita/myshoes](https://github.com/whywaita/myshoes).
It implements a JSON-RPC server for managing target data from the myshoes.

## Status

This project is in early development and not yet ready for production use.

## Installation

### Usage with VS Code

```json
{
    "mcp": {
        "servers": {
            "myshoes-mcp-server": {
                "type": "stdio",
                "command": "docker",
                "args": [
                    "run",
                    "-i",
                    "--rm",
                    "ghcr.io/whywaita/myshoes-mcp-server:latest",
                    "/myshoes-mcp-server",
                    "stdio",
                    "--host",
                    "http://192.0.2.10:8080"
                ]
            }
        }
    }
}
```