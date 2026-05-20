"""LiteLLM-rooted dspy.LM configuration factory.

Reads LITELLM_BASE_URL (default http://litellm:4000) and LITELLM_API_KEY
for the in-container API key (matches researcher gateway.py:26-27).
Also reads STORM_MODEL_OUTLINE and STORM_MODEL_ARTICLE.

# @MX:NOTE: [AUTO] LiteLLM proxy config; no direct vendor SDK imports.
"""

from __future__ import annotations

import os


class GatewayConfig:
    """LiteLLM gateway configuration sourced from environment variables.

    Matches the researcher gateway.py:26-27 env-var convention:
      LITELLM_BASE_URL (default http://litellm:4000)
      LITELLM_API_KEY  (default "")
    """

    def __init__(self) -> None:
        self.base_url = os.environ.get("LITELLM_BASE_URL", "http://litellm:4000")
        self.api_key = os.environ.get("LITELLM_API_KEY", "")
        self.model_outline = os.environ.get("STORM_MODEL_OUTLINE", "claude-haiku-4-5")
        self.model_article = os.environ.get("STORM_MODEL_ARTICLE", "claude-sonnet-4-6")

    def build_lm(self, model: str) -> dict[str, str]:
        """Build a dspy.LM config dict pointing at the LiteLLM proxy.

        Returns a dict with model, api_base, api_key suitable for
        constructing a dspy.LM or knowledge_storm.lm config.
        """
        return {
            "model": model,
            "api_base": self.base_url,
            "api_key": self.api_key,
        }
