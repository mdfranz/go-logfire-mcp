# /// script
# dependencies = [
#     "pydantic-ai",
#     "httpx",
# ]
# ///
"""
End-to-end MCP test harness for logfire-mcp using pydantic-ai.

Usage:
    uv run tools/test_mcp.py [model]

Examples:
    uv run tools/test_mcp.py                           # Deterministic offline test if no LLM key
    uv run tools/test_mcp.py google-gla:gemini-2.5-flash # Live LLM test with pydantic-ai

Requires: LOGFIRE_API_TOKEN, LOGFIRE_READ_TOKEN, or LOGFIRE_API_KEY in environment for live queries.
Requires: GOOGLE_API_KEY, GEMINI_API_KEY, or OPENAI_API_KEY for LLM agent execution.
"""

import asyncio
import logging
import os
import subprocess
import sys
import time
from pathlib import Path

from pydantic_ai import Agent
from pydantic_ai.mcp import MCPToolset, StdioTransport

logging.basicConfig(
    level=logging.DEBUG,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    handlers=[
        logging.FileHandler("logfire_mcp_test.log"),
        logging.StreamHandler(),
    ],
)
for h in logging.root.handlers:
    if type(h) is logging.StreamHandler:
        h.setLevel(logging.INFO)

logger = logging.getLogger("logfire_mcp_test")

REPO_ROOT = Path(__file__).resolve().parent.parent
SERVER_BINARY = REPO_ROOT / "logfire-mcp"
FIXTURE_PORT = 8902

SYSTEM_PROMPT = """
You are a telemetry analyst querying Logfire data using the logfire-mcp server.

Available tools and capabilities:
- query_run: Run DataFusion SQL queries against records and metrics tables. Requires query and min_timestamp (RFC3339).
- get_schema_metadata: Retrieve full schema documentation and example SQL queries.
- logfire://schema resource: Documentation of records/metrics schema fields.

When answering queries:
1. Always check schema details with get_schema_metadata if you are unsure of column names or types.
2. Formulate concise, read-only SQL queries with min_timestamp and limit.
3. Provide a clear summary of findings with sample data returned.
"""

TASKS = [
    (
        "Schema Inspection",
        "Retrieve the Logfire database schema metadata and list the primary columns in the records table.",
    ),
    (
        "Service Discovery",
        "Query the records table to list all distinct service names and their record counts since 2025-01-01T00:00:00Z.",
    ),
    (
        "Recent Telemetry",
        "Fetch the 5 most recent records from the records table, including start_timestamp, service_name, and message.",
    ),
]

def build_mcp_binary():
    cmd = ["go", "build", "-o", str(SERVER_BINARY), "./cmd/logfire-mcp"]
    res = subprocess.run(cmd, cwd=REPO_ROOT, capture_output=True, text=True)
    if res.returncode != 0:
        logger.error("Go build failed:\n%s", res.stderr)
        sys.exit(1)
    logger.info("Built binary %s", SERVER_BINARY)

def start_fixture_server():
    cmd = [sys.executable, str(REPO_ROOT / "tools" / "fixture_server.py"), str(FIXTURE_PORT)]
    proc = subprocess.Popen(cmd)
    time.sleep(0.5)
    return proc

async def run_deterministic_tests():
    """Runs deterministic protocol tests against the local fixture server."""
    logger.info("Running deterministic offline protocol tests against fixture server...")
    fixture_proc = start_fixture_server()
    env = {
        **os.environ,
        "LOGFIRE_BASE_URL": f"http://127.0.0.1:{FIXTURE_PORT}",
        "LOGFIRE_API_TOKEN": "dummy-test-token",
        "LOGFIRE_MCP_LOGFILE": "off",
    }
    
    transport = StdioTransport(command=str(SERVER_BINARY), args=[], env=env)
    async with MCPToolset(transport) as toolset:
        tools = await toolset.get_tools()
        tool_names = [t.name for t in tools]
        logger.info("Server tools exposed: %s", tool_names)
        assert "query_run" in tool_names
        assert "get_schema_metadata" in tool_names
        
        # Test get_schema_metadata
        schema_result = await toolset.call_tool("get_schema_metadata", {})
        assert "Logfire Schema Reference" in str(schema_result)
        logger.info("✓ get_schema_metadata verified")

        # Test query_run
        query_result = await toolset.call_tool(
            "query_run",
            {"query": "SELECT message FROM records", "min_timestamp": "2026-01-01T00:00:00Z"}
        )
        assert "hello from fixture" in str(query_result)
        logger.info("✓ query_run verified")

    fixture_proc.terminate()
    fixture_proc.wait()
    logger.info("All deterministic tests passed!")

async def run_agent_tests(model_name: str):
    """Runs live agent tests using pydantic-ai and logfire-mcp server."""
    logger.info("Running live agent tests with model %s...", model_name)
    
    token = os.getenv("LOGFIRE_API_TOKEN") or os.getenv("LOGFIRE_READ_TOKEN") or os.getenv("LOGFIRE_API_KEY")
    if not token:
        logger.error("LOGFIRE_API_TOKEN, LOGFIRE_READ_TOKEN, or LOGFIRE_API_KEY must be set for live agent tests")
        sys.exit(1)

    env = {
        **os.environ,
        "LOGFIRE_MCP_LOGFILE": "off",
    }

    transport = StdioTransport(command=str(SERVER_BINARY), args=[], env=env)
    async with MCPToolset(transport) as toolset:
        agent = Agent(
            model=model_name,
            system_prompt=SYSTEM_PROMPT,
            toolsets=[toolset],
        )

        for name, task_prompt in TASKS:
            print(f"\n========================================")
            print(f" Task: {name}")
            print(f"========================================")
            print(f"Prompt: {task_prompt.strip()}\n")
            
            result = await agent.run(task_prompt)
            print(f"Agent Answer:\n{result.output}\n")

def select_default_model() -> str | None:
    if "GOOGLE_API_KEY" in os.environ or "GEMINI_API_KEY" in os.environ:
        if "GEMINI_API_KEY" in os.environ and "GOOGLE_API_KEY" not in os.environ:
            os.environ["GOOGLE_API_KEY"] = os.environ["GEMINI_API_KEY"]
        return "google:gemini-2.5-flash"
    if "OPENAI_API_KEY" in os.environ:
        return "openai:gpt-4o-mini"
    if "ANTHROPIC_API_KEY" in os.environ:
        return "anthropic:claude-3-5-sonnet-latest"
    return None

async def main():
    build_mcp_binary()

    model = sys.argv[1] if len(sys.argv) > 1 else select_default_model()

    if model:
        await run_agent_tests(model)
    else:
        logger.info("No LLM API key detected. Falling back to deterministic offline test suite.")
        await run_deterministic_tests()

if __name__ == "__main__":
    asyncio.run(main())
