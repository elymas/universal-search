"""Tests for storm LiteLLM gateway configuration.

RED phase: Define expected gateway behavior before implementation.
Constraints: LITELLM_BASE_URL default http://litellm:4000, LITELLM_API_KEY for
in-container env (matches researcher gateway.py:26-27).
NO direct vendor SDK imports.
"""

from __future__ import annotations

import os
from unittest.mock import patch


class TestGatewayConfig:
    """Gateway reads LiteLLM configuration from environment."""

    def test_default_base_url(self) -> None:
        """Gateway defaults LITELLM_BASE_URL to http://litellm:4000."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {}, clear=True):
            cfg = GatewayConfig()
            assert cfg.base_url == "http://litellm:4000"

    def test_custom_base_url(self) -> None:
        """Gateway reads LITELLM_BASE_URL from environment."""
        from storm.gateway import GatewayConfig

        env = {"LITELLM_BASE_URL": "http://custom:5000"}
        with patch.dict(os.environ, env, clear=True):
            cfg = GatewayConfig()
            assert cfg.base_url == "http://custom:5000"

    def test_default_api_key_empty(self) -> None:
        """Gateway defaults LITELLM_API_KEY to empty string."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {}, clear=True):
            cfg = GatewayConfig()
            assert cfg.api_key == ""

    def test_custom_api_key(self) -> None:
        """Gateway reads LITELLM_API_KEY from environment."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {"LITELLM_API_KEY": "sk-test-key"}, clear=True):
            cfg = GatewayConfig()
            assert cfg.api_key == "sk-test-key"

    def test_storm_model_outline_default(self) -> None:
        """Gateway defaults STORM_MODEL_OUTLINE to claude-haiku-4-5."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {}, clear=True):
            cfg = GatewayConfig()
            assert cfg.model_outline == "claude-haiku-4-5"

    def test_storm_model_article_default(self) -> None:
        """Gateway defaults STORM_MODEL_ARTICLE to claude-sonnet-4-6."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {}, clear=True):
            cfg = GatewayConfig()
            assert cfg.model_article == "claude-sonnet-4-6"

    def test_custom_model_outline(self) -> None:
        """Gateway reads STORM_MODEL_OUTLINE from environment."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {"STORM_MODEL_OUTLINE": "gpt-4o"}, clear=True):
            cfg = GatewayConfig()
            assert cfg.model_outline == "gpt-4o"

    def test_custom_model_article(self) -> None:
        """Gateway reads STORM_MODEL_ARTICLE from environment."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {"STORM_MODEL_ARTICLE": "gpt-4o-mini"}, clear=True):
            cfg = GatewayConfig()
            assert cfg.model_article == "gpt-4o-mini"


class TestGatewayNoVendorImports:
    """Gateway must NOT import vendor SDKs directly."""

    def test_no_openai_import(self) -> None:
        """gateway.py must not import openai directly."""
        import importlib

        spec = importlib.util.find_spec("storm.gateway")
        assert spec is not None
        source = spec.loader.get_source("storm.gateway")  # type: ignore[union-attr]
        assert source is not None
        assert "import openai" not in source
        assert "from openai" not in source

    def test_no_anthropic_import(self) -> None:
        """gateway.py must not import anthropic directly."""
        import importlib

        spec = importlib.util.find_spec("storm.gateway")
        source = spec.loader.get_source("storm.gateway")  # type: ignore[union-attr]
        assert source is not None
        assert "import anthropic" not in source
        assert "from anthropic" not in source


class TestGatewayBuildLM:
    """Gateway.build_lm produces dspy.LM config dict."""

    def test_build_lm_returns_dict_with_model_and_base_url(self) -> None:
        """build_lm returns a dict with model, api_base, api_key."""
        from storm.gateway import GatewayConfig

        with patch.dict(os.environ, {"LITELLM_API_KEY": "sk-test"}, clear=True):
            cfg = GatewayConfig()
            result = cfg.build_lm("test-model")
            assert isinstance(result, dict)
            assert "model" in result
            assert result["model"] == "test-model"
            assert "api_base" in result
            assert "api_key" in result
            assert result["api_key"] == "sk-test"

    def test_build_lm_uses_base_url(self) -> None:
        """build_lm uses the configured base_url as api_base."""
        from storm.gateway import GatewayConfig

        with patch.dict(
            os.environ,
            {"LITELLM_BASE_URL": "http://custom:5000", "LITELLM_API_KEY": "k"},
            clear=True,
        ):
            cfg = GatewayConfig()
            result = cfg.build_lm("m")
            assert result["api_base"] == "http://custom:5000"
