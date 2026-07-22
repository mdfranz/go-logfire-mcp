# /// script
# dependencies = [
#     "pydantic",
#     "httpx",
#     "mcp",
# ]
# ///
import asyncio
import os
import subprocess
import sys
import time
from mcp import ClientSession, StdioServerParameters
from mcp.client.stdio import stdio_client

FIXTURE_PORT = 8902

def start_fixture_server():
    cmd = [sys.executable, "tools/fixture_server.py", str(FIXTURE_PORT)]
    proc = subprocess.Popen(cmd)
    time.sleep(0.5)
    return proc

def build_mcp_binary():
    cmd = ["go", "build", "-o", "bin/logfire-mcp", "./cmd/logfire-mcp"]
    res = subprocess.run(cmd, capture_output=True, text=True)
    if res.returncode != 0:
        print(f"Build failed:\n{res.stderr}", file=sys.stderr)
        sys.exit(1)

async def test_mcp_server():
    server_params = StdioServerParameters(
        command="./bin/logfire-mcp",
        args=[],
        env={
            **os.environ,
            "LOGFIRE_BASE_URL": f"http://127.0.0.1:{FIXTURE_PORT}",
            "LOGFIRE_API_TOKEN": "dummy-test-token",
            "LOGFIRE_MCP_LOGFILE": "off",
        }
    )

    async with stdio_client(server_params) as (read, write):
        async with ClientSession(read, write) as session:
            await session.initialize()

            # Test ListTools
            tools = await session.list_tools()
            tool_names = [t.name for t in tools.tools]
            assert "query_run" in tool_names, f"query_run missing from {tool_names}"
            assert "get_schema_metadata" in tool_names, f"get_schema_metadata missing from {tool_names}"
            print("✓ ListTools verified")

            # Test CallTool query_run
            result = await session.call_tool(
                "query_run",
                arguments={
                    "query": "SELECT message FROM records",
                    "min_timestamp": "2026-01-01T00:00:00Z"
                }
            )
            assert not result.isError, f"Tool execution failed: {result}"
            assert "hello from fixture" in result.content[0].text, f"Unexpected content: {result.content[0].text}"
            print("✓ query_run tool call verified")

            # Test CallTool get_schema_metadata
            schema_res = await session.call_tool("get_schema_metadata", arguments={})
            assert not schema_res.isError
            assert "Logfire Schema Reference" in schema_res.content[0].text
            print("✓ get_schema_metadata tool call verified")

            # Test ReadResource logfire://schema
            resource_res = await session.read_resource("logfire://schema")
            assert "Logfire Schema Reference" in resource_res.contents[0].text
            print("✓ logfire://schema resource read verified")

def main():
    os.makedirs("bin", exist_ok=True)
    build_mcp_binary()
    fixture_proc = start_fixture_server()
    try:
        asyncio.run(test_mcp_server())
        print("All E2E MCP tests passed successfully!")
    finally:
        fixture_proc.terminate()
        fixture_proc.wait()

if __name__ == "__main__":
    main()
