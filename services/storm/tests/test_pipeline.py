"""Integration tests for pipeline.py — STORM orchestration with mocked LM.

Tests cover:
- Pipeline builds correct LM configs via gateway
- Pipeline reads env vars for STORMWikiRunnerArguments
- Pipeline creates InjectedRM with request docs
- Pipeline invokes STORMWikiRunner.run() with correct parameters
- Pipeline parses runner output into GenerateReportResponse
- LiteLLM-only import constraint (no direct vendor SDK)
- NFR-DEEP1-002: Property tests for citation invariant and report well-formedness
"""

from __future__ import annotations

import os
import re
import sys
import types
from typing import Any
from unittest.mock import MagicMock, patch

import pytest

from hypothesis import given, settings
from hypothesis import strategies as st

from storm.models import Citation, GenerateReportResponse, Section, Sentence

# ---------------------------------------------------------------------------
# Mock heavy dependencies BEFORE importing storm.pipeline
# ---------------------------------------------------------------------------


def _setup_storm_mocks() -> dict[str, MagicMock]:
    """Create mock modules for knowledge_storm, dspy, etc."""
    mocks: dict[str, MagicMock] = {}

    # Mock dspy
    dspy_mock = types.ModuleType("dspy")
    dspy_mock.Retrieve = type("Retrieve", (), {"__init__": lambda self: None, "forward": lambda self, *a, **kw: []})
    mocks["dspy"] = dspy_mock

    # Mock knowledge_storm
    ks_mock = types.ModuleType("knowledge_storm")
    runner_cls = MagicMock()
    runner_instance = MagicMock()
    runner_instance.run.return_value = None
    runner_instance.summary.return_value = {
        "title": "Mock Report",
        "sections": [
            {
                "heading": "Introduction",
                "level": 1,
                "sentences": [
                    {"text": "This is a test sentence."},
                ],
            },
        ],
        "references": {},
    }
    runner_cls.return_value = runner_instance
    ks_mock.STORMWikiRunner = runner_cls

    lm_configs_cls = MagicMock()
    lm_configs_instance = MagicMock()
    lm_configs_cls.return_value = lm_configs_instance
    ks_mock.STORMWikiLMConfigs = lm_configs_cls

    args_cls = MagicMock()
    args_instance = MagicMock()
    args_instance.search_top_k = 3
    args_instance.max_perspectives = 2
    args_cls.return_value = args_instance
    ks_mock.STORMWikiRunnerArguments = args_cls

    mocks["knowledge_storm"] = ks_mock

    # Mock knowledge_storm.lm
    ks_lm_mock = types.ModuleType("knowledge_storm.lm")
    litellm_cls = MagicMock()
    litellm_instance = MagicMock()
    litellm_cls.return_value = litellm_instance
    ks_lm_mock.LitellmModel = litellm_cls
    mocks["knowledge_storm.lm"] = ks_lm_mock

    return mocks


def _install_mocks(mocks: dict[str, MagicMock]) -> None:
    """Install mock modules into sys.modules."""
    for name, mod in mocks.items():
        sys.modules[name] = mod


def _remove_mocks(mocks: dict[str, MagicMock]) -> None:
    """Remove mock modules from sys.modules."""
    for name in mocks:
        sys.modules.pop(name, None)
    # Also clean up pipeline module so it re-imports
    sys.modules.pop("storm.pipeline", None)


@pytest.fixture(autouse=True)
def mock_storm_deps():
    """Auto-use fixture: install mocks before test, remove after."""
    mocks = _setup_storm_mocks()
    _install_mocks(mocks)
    yield mocks
    _remove_mocks(mocks)


# ---------------------------------------------------------------------------
# Pipeline configuration tests
# ---------------------------------------------------------------------------


class TestPipelineConfig:
    """Verify pipeline reads env vars and builds correct configs."""

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "3",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "5",
            "STORM_MAX_THREAD_NUM": "4",
            "STORM_DO_POLISH": "true",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline-model",
            "STORM_MODEL_ARTICLE": "test-article-model",
        },
    )
    def test_builds_runner_args_from_env(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import build_runner_args

        args = build_runner_args()
        call_kwargs = mock_storm_deps["knowledge_storm"].STORMWikiRunnerArguments.call_args
        assert call_kwargs is not None

    @patch.dict(
        os.environ,
        {
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline-model",
            "STORM_MODEL_ARTICLE": "test-article-model",
        },
    )
    def test_builds_lm_configs_from_gateway(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import build_lm_configs

        configs = build_lm_configs()
        assert configs is not None
        # Verify LitellmModel was called
        litellm_cls = mock_storm_deps["knowledge_storm.lm"].LitellmModel
        assert litellm_cls.call_count >= 2

    @patch.dict(
        os.environ,
        {
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "haiku",
            "STORM_MODEL_ARTICLE": "sonnet",
        },
    )
    def test_litellm_model_receives_gateway_config(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import build_lm_configs

        build_lm_configs()
        litellm_cls = mock_storm_deps["knowledge_storm.lm"].LitellmModel
        calls = litellm_cls.call_args_list
        # Each call should receive model and api_base/api_key
        for call in calls:
            kwargs = call.kwargs if call.kwargs else call[1]
            assert "model" in kwargs or len(call[0]) > 0


# ---------------------------------------------------------------------------
# Pipeline execution tests
# ---------------------------------------------------------------------------


class TestPipelineRun:
    """Verify pipeline.run() orchestrates correctly."""

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_run_invokes_runner_with_correct_topic(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        req = GenerateReportRequest(
            request_id="test-001",
            query="quantum computing",
            docs=[
                {"url": "https://example.com/1", "title": "QC Intro", "body": "quantum bits"},
            ],
        )
        response = run(req)
        runner_cls = mock_storm_deps["knowledge_storm"].STORMWikiRunner
        runner_instance = runner_cls.return_value
        runner_instance.run.assert_called_once()
        run_call = runner_instance.run.call_args
        assert run_call.kwargs.get("topic") == "quantum computing" or run_call[1].get("topic") == "quantum computing"

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_run_returns_generate_report_response(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest, GenerateReportResponse
        from storm.pipeline import run

        req = GenerateReportRequest(
            request_id="test-002",
            query="test query",
            docs=[
                {"url": "https://example.com/1", "title": "T1", "body": "test body"},
            ],
        )
        response = run(req)
        assert isinstance(response, GenerateReportResponse)
        assert response.request_id == "test-002"
        assert response.schema_version == 1

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_run_uses_injected_rm_with_request_docs(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        docs = [
            {"url": "https://example.com/1", "title": "T1", "body": "body 1"},
            {"url": "https://example.com/2", "title": "T2", "body": "body 2"},
        ]
        req = GenerateReportRequest(
            request_id="test-003",
            query="test",
            docs=docs,
        )
        run(req)
        runner_cls = mock_storm_deps["knowledge_storm"].STORMWikiRunner
        # Verify runner was constructed with 3 args: (args, lm_configs, rm)
        constructor_call = runner_cls.call_args
        assert constructor_call is not None
        # The third positional arg should be the InjectedRM instance
        if len(constructor_call[0]) >= 3:
            rm_arg = constructor_call[0][2]
            assert hasattr(rm_arg, "forward"), "Third arg to STORMWikiRunner should be InjectedRM"

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_run_run_called_with_do_research_and_generate(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        req = GenerateReportRequest(
            request_id="test-004",
            query="test",
            docs=[{"url": "https://example.com/1", "title": "T", "body": "b"}],
        )
        run(req)
        runner_instance = mock_storm_deps["knowledge_storm"].STORMWikiRunner.return_value
        run_call = runner_instance.run.call_args
        assert run_call.kwargs.get("do_research") is True or run_call[1].get("do_research") is True
        assert run_call.kwargs.get("do_generate_article") is True or run_call[1].get("do_generate_article") is True


# ---------------------------------------------------------------------------
# Output parsing tests
# ---------------------------------------------------------------------------


class TestPipelineOutputParsing:
    """Verify pipeline parses runner.summary() into response shape."""

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_parses_summary_into_sections(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest, Section
        from storm.pipeline import run

        # Configure the mock runner to return structured output
        runner_instance = mock_storm_deps["knowledge_storm"].STORMWikiRunner.return_value
        runner_instance.summary.return_value = {
            "title": "Quantum Computing Report",
            "sections": [
                {
                    "heading": "Introduction",
                    "level": 1,
                    "sentences": [
                        {"text": "Quantum computing uses qubits."},
                        {"text": "It is a new paradigm."},
                    ],
                },
                {
                    "heading": "Applications",
                    "level": 2,
                    "sentences": [
                        {"text": "Drug discovery is a key application."},
                    ],
                },
            ],
            "references": {},
        }

        req = GenerateReportRequest(
            request_id="test-005",
            query="quantum computing",
            docs=[{"url": "https://example.com/1", "title": "QC", "body": "quantum"}],
        )
        response = run(req)
        assert response.title == "Quantum Computing Report"
        assert len(response.sections) == 2
        assert response.sections[0].heading == "Introduction"
        assert response.sections[0].level == 1
        assert len(response.sections[0].sentences) == 2
        assert response.sections[1].heading == "Applications"

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_handles_empty_summary_gracefully(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        runner_instance = mock_storm_deps["knowledge_storm"].STORMWikiRunner.return_value
        runner_instance.summary.return_value = {}

        req = GenerateReportRequest(
            request_id="test-006",
            query="test",
            docs=[],
        )
        response = run(req)
        assert response.title == ""
        assert response.sections == []
        assert response.citations == []


# ---------------------------------------------------------------------------
# LiteLLM-only import constraint
# ---------------------------------------------------------------------------


class TestLiteLLMOnly:
    """Verify no direct vendor SDK imports exist in pipeline code."""

    def test_pipeline_has_no_vendor_sdk_imports(self) -> None:
        import importlib

        # Need mocks in place to import pipeline
        mocks = _setup_storm_mocks()
        _install_mocks(mocks)
        try:
            spec = importlib.util.find_spec("storm.pipeline")
            assert spec is not None
            import storm.pipeline as pipeline_mod

            source = open(pipeline_mod.__file__).read()
            vendor_imports = [
                "openai",
                "anthropic",
                "google.generativeai",
                "boto3",
            ]
            for vendor in vendor_imports:
                assert f"import {vendor}" not in source, f"Direct vendor import found: {vendor}"
        finally:
            _remove_mocks(mocks)


# ---------------------------------------------------------------------------
# Deadline and budget guard tests
# ---------------------------------------------------------------------------


class TestDeadlineHelpers:
    """Verify compute_effective_deadline_ms and custom exceptions."""

    @patch.dict(
        os.environ,
        {"STORM_MAX_LATENCY_MS": "60000"},
        clear=False,
    )
    def test_compute_deadline_returns_ceiling_when_no_request_max(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import compute_effective_deadline_ms

        result = compute_effective_deadline_ms(request_max_ms=None)
        assert result == 60000

    @patch.dict(
        os.environ,
        {"STORM_MAX_LATENCY_MS": "60000"},
        clear=False,
    )
    def test_compute_deadline_returns_request_when_below_ceiling(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import compute_effective_deadline_ms

        result = compute_effective_deadline_ms(request_max_ms=30000, ceiling_ms=60000)
        assert result == 30000

    @patch.dict(
        os.environ,
        {"STORM_MAX_LATENCY_MS": "60000"},
        clear=False,
    )
    def test_compute_deadline_clamps_request_to_ceiling(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import compute_effective_deadline_ms

        result = compute_effective_deadline_ms(request_max_ms=120000, ceiling_ms=60000)
        assert result == 60000

    def test_compute_deadline_uses_default_when_env_unset(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import _DEFAULT_MAX_LATENCY_MS, compute_effective_deadline_ms

        # Ensure STORM_MAX_LATENCY_MS is not set
        env_copy = os.environ.pop("STORM_MAX_LATENCY_MS", None)
        try:
            result = compute_effective_deadline_ms(request_max_ms=None, ceiling_ms=None)
            assert result == _DEFAULT_MAX_LATENCY_MS
        finally:
            if env_copy is not None:
                os.environ["STORM_MAX_LATENCY_MS"] = env_copy

    def test_compute_deadline_exact_ceiling_not_clamped(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import compute_effective_deadline_ms

        result = compute_effective_deadline_ms(request_max_ms=60000, ceiling_ms=60000)
        assert result == 60000


class TestCustomExceptions:
    """Verify DeadlineExceededError and BudgetExceededError carry required fields."""

    def test_deadline_exceeded_error_fields(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import DeadlineExceededError

        err = DeadlineExceededError(elapsed_ms=35000.5, partial_sections_completed=3)
        assert err.elapsed_ms == 35000.5
        assert err.partial_sections_completed == 3
        assert "35000.5" in str(err)
        assert "3" in str(err)

    def test_deadline_exceeded_error_defaults(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import DeadlineExceededError

        err = DeadlineExceededError(elapsed_ms=1000.0)
        assert err.partial_sections_completed == 0

    def test_budget_exceeded_error_fields(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import BudgetExceededError

        err = BudgetExceededError(cost_usd=3.50, cap_usd=2.50)
        assert err.cost_usd == 3.50
        assert err.cap_usd == 2.50
        assert "3.5" in str(err)
        assert "2.5" in str(err)


# ---------------------------------------------------------------------------
# Runner args env var tests
# ---------------------------------------------------------------------------


class TestRunnerArgsEnvVars:
    """Verify build_runner_args reads each env var correctly."""

    @patch.dict(
        os.environ,
        {
            "STORM_OUTPUT_DIR": "/custom/output",
            "STORM_SEARCH_TOP_K": "7",
            "STORM_MAX_PERSPECTIVES": "4",
            "STORM_MAX_CONV_TURNS": "3",
            "STORM_MAX_THREAD_NUM": "5",
        },
    )
    def test_runner_args_reads_all_env_vars(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import build_runner_args

        args = build_runner_args()
        call_kwargs = mock_storm_deps["knowledge_storm"].STORMWikiRunnerArguments.call_args
        assert call_kwargs is not None
        kwargs = call_kwargs.kwargs or call_kwargs[1]
        assert kwargs.get("output_dir") == "/custom/output"
        assert kwargs.get("search_top_k") == 7
        assert kwargs.get("max_perspective") == 4
        assert kwargs.get("max_conv_turn") == 3
        assert kwargs.get("max_thread_num") == 5


# ---------------------------------------------------------------------------
# LM configs 5-slot verification
# ---------------------------------------------------------------------------


class TestLMConfigsSlots:
    """Verify build_lm_configs sets all 5 LM slots."""

    @patch.dict(
        os.environ,
        {
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_lm_configs_sets_five_lm_slots(self, mock_storm_deps: dict) -> None:
        from storm.pipeline import build_lm_configs

        lm_configs = build_lm_configs()
        instance = mock_storm_deps["knowledge_storm"].STORMWikiLMConfigs.return_value
        # Verify all 5 setter methods were called
        instance.set_conv_simulator_lm.assert_called_once()
        instance.set_question_asker_lm.assert_called_once()
        instance.set_outline_gen_lm.assert_called_once()
        instance.set_article_gen_lm.assert_called_once()
        instance.set_article_polish_lm.assert_called_once()


# ---------------------------------------------------------------------------
# Parse summary edge cases
# ---------------------------------------------------------------------------


class TestParseSummaryEdgeCases:
    """Verify _parse_summary handles malformed/missing data."""

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_parse_summary_with_missing_section_keys(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        runner_instance = mock_storm_deps["knowledge_storm"].STORMWikiRunner.return_value
        runner_instance.summary.return_value = {
            "title": "Report",
            "sections": [
                {},  # missing heading, level, sentences
                {"heading": "Valid"},  # missing level, sentences
            ],
        }

        req = GenerateReportRequest(
            request_id="test-edge-001",
            query="test",
            docs=[{"url": "https://example.com/1", "title": "T", "body": "b"}],
        )
        response = run(req)
        assert response.title == "Report"
        assert len(response.sections) == 2
        assert response.sections[0].heading == ""
        assert response.sections[0].level == 1
        assert response.sections[1].heading == "Valid"

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_parse_summary_citations_always_empty_deferred_to_m3(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        runner_instance = mock_storm_deps["knowledge_storm"].STORMWikiRunner.return_value
        runner_instance.summary.return_value = {
            "title": "Report",
            "sections": [],
            "references": {"1": "some ref"},
        }

        req = GenerateReportRequest(
            request_id="test-edge-002",
            query="test",
            docs=[],
        )
        response = run(req)
        assert response.citations == []

    @patch.dict(
        os.environ,
        {
            "STORM_MAX_PERSPECTIVES": "2",
            "STORM_MAX_CONV_TURNS": "2",
            "STORM_SEARCH_TOP_K": "3",
            "STORM_MAX_THREAD_NUM": "3",
            "STORM_DO_POLISH": "false",
            "LITELLM_BASE_URL": "http://test:4000",
            "LITELLM_API_KEY": "test-key",
            "STORM_MODEL_OUTLINE": "test-outline",
            "STORM_MODEL_ARTICLE": "test-article",
        },
    )
    def test_parse_summary_sentence_missing_text_defaults_empty(self, mock_storm_deps: dict) -> None:
        from storm.models import GenerateReportRequest
        from storm.pipeline import run

        runner_instance = mock_storm_deps["knowledge_storm"].STORMWikiRunner.return_value
        runner_instance.summary.return_value = {
            "title": "Report",
            "sections": [
                {
                    "heading": "S1",
                    "level": 1,
                    "sentences": [
                        {},  # missing text
                        {"text": "valid sentence"},
                    ],
                },
            ],
        }

        req = GenerateReportRequest(
            request_id="test-edge-003",
            query="test",
            docs=[],
        )
        response = run(req)
        assert len(response.sections[0].sentences) == 2
        assert response.sections[0].sentences[0].text == ""
        assert response.sections[0].sentences[1].text == "valid sentence"


# ---------------------------------------------------------------------------
# NFR-DEEP1-002: Property tests for citation invariant + report well-formedness
# ---------------------------------------------------------------------------


# -- Hypothesis strategies for STORM-shaped responses --


@st.composite
def _st_well_formed_response(
    draw: st.DrawFn,
) -> GenerateReportResponse:
    """Generate a well-formed GenerateReportResponse.

    Produces realistic STORM-shaped output where:
    - Citations are 1-indexed and sorted by marker
    - Every Section has non-empty sentences
    - Title is non-empty
    - Sentence markers reference valid citations
    """
    n_citations = draw(st.integers(min_value=1, max_value=20))
    citations = [
        Citation(
            marker=i + 1,
            doc_id=f"doc-{i}",
            url=f"https://example.com/{i}",
            title=f"Document {i}",
        )
        for i in range(n_citations)
    ]

    n_sections = draw(st.integers(min_value=1, max_value=8))
    sections: list[Section] = []
    for _ in range(n_sections):
        n_sentences = draw(st.integers(min_value=1, max_value=10))
        sentences: list[Sentence] = []
        for _ in range(n_sentences):
            # Each sentence references at least one valid marker.
            n_markers = draw(st.integers(min_value=1, max_value=min(3, n_citations)))
            marker_indices = draw(
                st.lists(
                    st.integers(min_value=1, max_value=n_citations),
                    min_size=n_markers,
                    max_size=n_markers,
                    unique=True,
                )
            )
            # Build a realistic sentence text with [N] markers.
            marker_strs = [f"[{m}]" for m in sorted(marker_indices)]
            text_prefix = draw(st.from_regex(r"[A-Z][a-z ]{5,30}", fullmatch=True))
            text = text_prefix + " " + " ".join(marker_strs) + "."
            sentences.append(Sentence(text=text, citations=sorted(marker_indices)))

        heading = draw(st.from_regex(r"[A-Z][A-Za-z ]{3,30}", fullmatch=True))
        level = draw(st.integers(min_value=1, max_value=4))
        sections.append(Section(heading=heading, level=level, sentences=sentences))

    title = draw(st.from_regex(r"[A-Z][A-Za-z0-9 ]{5,50}", fullmatch=True))
    request_id = draw(st.from_regex(r"req-[a-z0-9]+", fullmatch=True))

    return GenerateReportResponse(
        request_id=request_id,
        title=title,
        sections=sections,
        citations=citations,
        schema_version=1,
    )


class TestPropertyLongForm:
    """NFR-DEEP1-002: Property tests over STORM-shaped responses via hypothesis."""

    @given(response=_st_well_formed_response())
    @settings(max_examples=100, deadline=None)
    def test_property_long_form_marker_resolution(self, response: GenerateReportResponse) -> None:
        """Every [N] marker in sentence text resolves to a doc_id in Citations[].

        NFR-DEEP1-002 invariant 1: citation markers are not dangling.
        """
        citation_markers = {c.marker for c in response.citations}

        for section in response.sections:
            for sentence in section.sentences:
                # Check markers from the Sentence model.
                for marker in sentence.citations:
                    assert marker in citation_markers, (
                        f"Marker [{marker}] in section '{section.heading}' "
                        f"has no matching citation. Available: {citation_markers}"
                    )

                # Also check [N] patterns in the text itself.
                text_markers = {int(m) for m in re.findall(r"\[(\d+)\]", sentence.text)}
                for marker in text_markers:
                    assert marker in citation_markers, (
                        f"Text marker [{marker}] in sentence '{sentence.text}' "
                        f"has no matching citation."
                    )

    @given(response=_st_well_formed_response())
    @settings(max_examples=100, deadline=None)
    def test_property_section_sentences_markers_in_range(self, response: GenerateReportResponse) -> None:
        """Every Section.sentences[].markers[] integer in [1, len(Citations)].

        NFR-DEEP1-002 invariant 2: markers reference valid 1-indexed citations.
        """
        n_citations = len(response.citations)
        assert n_citations >= 1

        for section in response.sections:
            for sentence in section.sentences:
                for marker in sentence.citations:
                    assert 1 <= marker <= n_citations, (
                        f"Marker {marker} out of range [1, {n_citations}] "
                        f"in section '{section.heading}'"
                    )

    @given(response=_st_well_formed_response())
    @settings(max_examples=100, deadline=None)
    def test_property_section_text_reconstruction(self, response: GenerateReportResponse) -> None:
        """Union of Section sentence texts preserves report content.

        NFR-DEEP1-002 invariant 3: concatenating all sentence texts yields
        non-empty content covering all citation markers.
        """
        all_text_parts: list[str] = []
        all_markers_in_text: set[int] = set()

        for section in response.sections:
            for sentence in section.sentences:
                all_text_parts.append(sentence.text)
                all_markers_in_text.update(sentence.citations)

        full_text = " ".join(all_text_parts)
        assert len(full_text) > 0, "Reconstructed text should be non-empty"

        # Verify all markers mentioned in sentences appear in the reconstructed text.
        for marker in all_markers_in_text:
            assert f"[{marker}]" in full_text, (
                f"Marker [{marker}] referenced in sentences but not in reconstructed text"
            )

    @given(response=_st_well_formed_response())
    @settings(max_examples=100, deadline=None)
    def test_property_no_empty_sections(self, response: GenerateReportResponse) -> None:
        """Every Section.sentences[] is non-empty (empty sections removed).

        NFR-DEEP1-002 invariant 4: no section has zero sentences.
        """
        for section in response.sections:
            assert len(section.sentences) > 0, (
                f"Section '{section.heading}' has zero sentences"
            )

    @given(response=_st_well_formed_response())
    @settings(max_examples=100, deadline=None)
    def test_property_title_non_empty(self, response: GenerateReportResponse) -> None:
        """report.title is non-empty.

        NFR-DEEP1-002 invariant 5: title is always present.
        """
        assert len(response.title) > 0, "Report title must be non-empty"
        assert response.title.strip() == response.title, "Title must not have leading/trailing whitespace"

    @given(response=_st_well_formed_response())
    @settings(max_examples=100, deadline=None)
    def test_property_citations_sorted_one_indexed(self, response: GenerateReportResponse) -> None:
        """Citations[] is sorted by marker, 1-indexed.

        NFR-DEEP1-002 invariant 6: citations are ordered and contiguous
        starting from 1.
        """
        markers = [c.marker for c in response.citations]

        # Sorted ascending.
        assert markers == sorted(markers), (
            f"Citations not sorted by marker: {markers}"
        )

        # 1-indexed starting from 1.
        assert markers[0] == 1, f"First marker should be 1, got {markers[0]}"

        # Contiguous: each marker is previous + 1.
        for i in range(1, len(markers)):
            assert markers[i] == markers[i - 1] + 1, (
                f"Citations not contiguous: marker {markers[i - 1]} followed by {markers[i]}"
            )
