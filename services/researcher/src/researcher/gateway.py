"""OpenAI SDK gateway wired to LiteLLM proxy.

REQ-SYN-002: Routes all LLM traffic through LITELLM_BASE_URL.
NFR-SYN-004: Extracts x-litellm-response-cost header → cost_usd.
"""

from __future__ import annotations

import logging
import os
from typing import Any

import httpx

logger = logging.getLogger(__name__)


class Gateway:
    """Thin wrapper around the OpenAI HTTP API, pointed at LiteLLM.

    # @MX:ANCHOR: [AUTO] LLM gateway; called by synthesis.synthesize
    # @MX:REASON: Public API boundary between synthesis logic and LLM transport.
    """

    def __init__(self, http_transport: httpx.AsyncBaseTransport | None = None) -> None:
        base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")
        api_key = os.environ.get("LITELLM_API_KEY", "")

        self._base_url = base_url.rstrip("/")
        self._api_key = api_key

        # Allow injecting a custom transport for testing (httpx mock)
        self._transport = http_transport

    async def complete(
        self,
        messages: list[dict[str, str]],
        model: str,
        lang: str,
    ) -> tuple[str, float, dict[str, int], str, str]:
        """Call the LiteLLM chat completions endpoint.

        # @MX:WARN: [AUTO] HTTP call to external service; can raise ConnectError
        # @MX:REASON: ConnectError must be caught by callers for degraded-mode fallback.

        Returns:
            (text, cost_usd, usage_dict, provider, model_used)
        """
        url = f"{self._base_url}/v1/chat/completions"
        payload: dict[str, Any] = {
            "model": model,
            "messages": messages,
        }
        headers = {
            "Authorization": f"Bearer {self._api_key}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient(transport=self._transport) as client:
            response = await client.post(url, json=payload, headers=headers)
            response.raise_for_status()

        data = response.json()
        text = data["choices"][0]["message"]["content"]
        usage = data.get("usage", {})
        usage_dict = {
            "prompt_tokens": usage.get("prompt_tokens", 0),
            "completion_tokens": usage.get("completion_tokens", 0),
        }
        model_used = data.get("model", model)
        # LiteLLM puts provider in model name prefix (e.g., "anthropic/claude-haiku")
        provider = model_used.split("/")[0] if "/" in model_used else "anthropic"

        # NFR-SYN-004: Extract cost from response header
        cost_usd = _extract_cost(response)

        return text, cost_usd, usage_dict, provider, model_used


def _extract_cost(response: httpx.Response) -> float:
    """Extract cost from x-litellm-response-cost header.

    Returns 0.0 on missing header (DEBUG log) or malformed value (WARN log).

    # @MX:NOTE: [AUTO] Header name: x-litellm-response-cost per SPEC-LLM-001 REQ-LLM-006.
    """
    raw = response.headers.get("x-litellm-response-cost")
    if raw is None:
        logger.debug("x-litellm-response-cost header absent; defaulting cost_usd to 0.0")
        return 0.0
    try:
        return float(raw)
    except (ValueError, TypeError):
        logger.warning(
            "Malformed x-litellm-response-cost header value %r; defaulting to 0.0",
            raw,
        )
        return 0.0
