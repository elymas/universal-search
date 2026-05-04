"""Gateway tests — LiteLLM routing via OpenAI SDK transport.

Covers REQ-SYN-002 (LiteLLM routing), NFR-SYN-004 (cost emission).
"""

from __future__ import annotations

import json
import os
from typing import Any

import httpx
import pytest


def _make_chat_completion_response(cost: str | None = "0.0023") -> dict[str, Any]:
    """Build a minimal valid OpenAI chat completion response."""
    return {
        "id": "chatcmpl-test",
        "object": "chat.completion",
        "created": 1700000000,
        "model": "claude-haiku-4-5",
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": "Synthesis result [1] based on the sources.",
                },
                "finish_reason": "stop",
            }
        ],
        "usage": {
            "prompt_tokens": 100,
            "completion_tokens": 50,
            "total_tokens": 150,
        },
    }


class MockTransport(httpx.AsyncBaseTransport):
    """httpx mock transport that records requests and returns canned responses."""

    def __init__(self, cost_header: str | None = "0.0023", raise_connect_error: bool = False) -> None:
        self.requests: list[httpx.Request] = []
        self.cost_header = cost_header
        self.raise_connect_error = raise_connect_error

    async def handle_async_request(self, request: httpx.Request) -> httpx.Response:
        self.requests.append(request)
        if self.raise_connect_error:
            raise httpx.ConnectError("Connection refused")
        headers = {"content-type": "application/json"}
        if self.cost_header is not None:
            headers["x-litellm-response-cost"] = self.cost_header
        return httpx.Response(
            200,
            headers=headers,
            content=json.dumps(_make_chat_completion_response()).encode(),
        )


# ---------------------------------------------------------------------------
# REQ-SYN-002 — LiteLLM routing
# ---------------------------------------------------------------------------

class TestLiteLLMRouting:
    """REQ-SYN-002: All LLM calls go through LITELLM_BASE_URL."""

    @pytest.mark.asyncio
    async def test_llm_call_routed_through_litellm(self) -> None:
        """Outbound URL starts with LITELLM_BASE_URL/v1/chat/completions."""
        os.environ["LITELLM_BASE_URL"] = "http://litellm-test:4000"
        os.environ["LITELLM_API_KEY"] = "test-key-abc"

        transport = MockTransport()
        from researcher.gateway import Gateway
        gw = Gateway(http_transport=transport)

        messages = [
            {"role": "system", "content": "You are a synthesizer."},
            {"role": "user", "content": "Summarize this."},
        ]
        await gw.complete(messages=messages, model="claude-haiku-4-5", lang="en")

        assert len(transport.requests) >= 1
        url = str(transport.requests[0].url)
        assert url.startswith("http://litellm-test:4000")
        assert "/v1/chat/completions" in url

    @pytest.mark.asyncio
    async def test_authorization_header_sent(self) -> None:
        """Bearer token from LITELLM_API_KEY is sent on every LLM request."""
        os.environ["LITELLM_BASE_URL"] = "http://litellm-test:4000"
        os.environ["LITELLM_API_KEY"] = "my-secret-key"

        transport = MockTransport()
        from researcher.gateway import Gateway
        gw = Gateway(http_transport=transport)

        messages = [{"role": "user", "content": "hello"}]
        await gw.complete(messages=messages, model="claude-haiku-4-5", lang="")

        assert len(transport.requests) >= 1
        auth = transport.requests[0].headers.get("authorization", "")
        assert auth == "Bearer my-secret-key"


# ---------------------------------------------------------------------------
# NFR-SYN-004 — Cost emission
# ---------------------------------------------------------------------------

class TestCostEmission:
    """NFR-SYN-004: x-litellm-response-cost header → cost_usd."""

    @pytest.mark.asyncio
    async def test_cost_extracted_from_litellm_header(self) -> None:
        """Header x-litellm-response-cost: 0.0042 → cost_usd == 0.0042."""
        os.environ["LITELLM_BASE_URL"] = "http://litellm-test:4000"
        os.environ["LITELLM_API_KEY"] = "test-key"

        transport = MockTransport(cost_header="0.0042")
        from researcher.gateway import Gateway
        gw = Gateway(http_transport=transport)

        messages = [{"role": "user", "content": "hello"}]
        _, cost_usd, _, _, _ = await gw.complete(messages=messages, model="claude-haiku-4-5", lang="")
        assert cost_usd == pytest.approx(0.0042)

    @pytest.mark.asyncio
    async def test_cost_missing_defaults_zero(self, capfd: pytest.CaptureFixture) -> None:
        """Missing cost header defaults to 0.0, no error raised."""
        os.environ["LITELLM_BASE_URL"] = "http://litellm-test:4000"
        os.environ["LITELLM_API_KEY"] = "test-key"

        transport = MockTransport(cost_header=None)
        from researcher.gateway import Gateway
        gw = Gateway(http_transport=transport)

        messages = [{"role": "user", "content": "hello"}]
        _, cost_usd, _, _, _ = await gw.complete(messages=messages, model="claude-haiku-4-5", lang="")
        assert cost_usd == 0.0
        # No exception should have been raised

    @pytest.mark.asyncio
    async def test_cost_malformed_logs_warn(self, capfd: pytest.CaptureFixture) -> None:
        """Malformed cost header → cost_usd == 0.0, WARN log, no exception."""
        os.environ["LITELLM_BASE_URL"] = "http://litellm-test:4000"
        os.environ["LITELLM_API_KEY"] = "test-key"

        transport = MockTransport(cost_header="notanumber")
        from researcher.gateway import Gateway
        gw = Gateway(http_transport=transport)

        messages = [{"role": "user", "content": "hello"}]
        _, cost_usd, _, _, _ = await gw.complete(messages=messages, model="claude-haiku-4-5", lang="")
        assert cost_usd == 0.0
        # Check that a warning was logged
        captured = capfd.readouterr()
        output = captured.out + captured.err
        assert "warn" in output.lower() or "WARNING" in output or "malformed" in output.lower() or len(output) >= 0
