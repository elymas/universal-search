"""STORM pipeline orchestration.

Builds LM configs via the LiteLLM gateway, assembles runner arguments
from environment variables, constructs an InjectedRM from the request
payload, and invokes the STORMWikiRunner.

# @MX:ANCHOR: [AUTO] Core STORM pipeline entry point.
# @MX:REASON: Single orchestration path for all /generate_report requests.
# @MX:SPEC: SPEC-DEEP-001 M2
"""

from __future__ import annotations

import logging
import os
from typing import TYPE_CHECKING, Any

from storm.models import (
    GenerateReportRequest,
    GenerateReportResponse,
    Section,
    Sentence,
)
from storm.obs import Timer

if TYPE_CHECKING:
    from knowledge_storm import (
        STORMWikiLMConfigs,
        STORMWikiRunnerArguments,
    )

    from storm.gateway import GatewayConfig

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Custom exceptions (REQ-DEEP1-004)
# ---------------------------------------------------------------------------


class DeadlineExceededError(Exception):
    """Raised when pipeline execution exceeds the effective deadline.

    # @MX:WARN: [AUTO] Deadline guard — must carry elapsed_ms and partial count.
    # @MX:REASON: Caller (app.py) uses these fields to build the 504 body.
    """

    def __init__(
        self,
        elapsed_ms: float,
        partial_sections_completed: int = 0,
    ) -> None:
        self.elapsed_ms = elapsed_ms
        self.partial_sections_completed = partial_sections_completed
        super().__init__(
            f"deadline_exceeded: elapsed_ms={elapsed_ms}, "
            f"partial_sections_completed={partial_sections_completed}"
        )


class BudgetExceededError(Exception):
    """Raised when cumulative LM cost exceeds the budget cap.

    # @MX:WARN: [AUTO] Budget guard — must carry cost_usd and cap_usd.
    # @MX:REASON: Caller (app.py) uses these fields to build the 402 body.
    """

    def __init__(
        self,
        cost_usd: float,
        cap_usd: float,
    ) -> None:
        self.cost_usd = cost_usd
        self.cap_usd = cap_usd
        super().__init__(
            f"budget_exceeded: cost_usd={cost_usd}, cap_usd={cap_usd}"
        )


# ---------------------------------------------------------------------------
# Deadline helpers
# ---------------------------------------------------------------------------

_DEFAULT_MAX_LATENCY_MS = 300_000
_DEFAULT_MAX_COST_USD = 2.50


def compute_effective_deadline_ms(
    request_max_ms: int | None,
    ceiling_ms: int | None = None,
) -> int:
    """Compute effective deadline in ms, clamped to the ceiling.

    REQ-DEEP1-004: min(request.max_latency_ms, STORM_MAX_LATENCY_MS).
    Emits a WARN log when the per-call override is clamped.

    # @MX:NOTE: [AUTO] Per-call deadline clamping with structured warning.
    """
    if ceiling_ms is None:
        ceiling_ms = int(
            os.environ.get("STORM_MAX_LATENCY_MS", str(_DEFAULT_MAX_LATENCY_MS))
        )

    if request_max_ms is None:
        return ceiling_ms

    if request_max_ms > ceiling_ms:
        logger.warning(
            {
                "reason": "max_latency_ms_clamped",
                "request_max_ms": request_max_ms,
                "ceiling_ms": ceiling_ms,
                "effective_ms": ceiling_ms,
            }
        )
        return ceiling_ms

    return request_max_ms


def build_lm_configs(gateway: GatewayConfig | None = None) -> STORMWikiLMConfigs:
    """Build STORM LM configs using LiteLLM proxy via gateway.

    # @MX:NOTE: [AUTO] LiteLLM proxy config; no direct vendor SDK imports.
    """
    from knowledge_storm import STORMWikiLMConfigs
    from knowledge_storm.lm import LitellmModel

    from storm.gateway import GatewayConfig

    if gateway is None:
        gateway = GatewayConfig()

    outline_lm = LitellmModel(
        model=gateway.model_outline,
        api_base=gateway.base_url,
        api_key=gateway.api_key,
    )
    article_lm = LitellmModel(
        model=gateway.model_article,
        api_base=gateway.base_url,
        api_key=gateway.api_key,
    )

    lm_configs = STORMWikiLMConfigs()
    lm_configs.set_conv_simulator_lm(outline_lm)
    lm_configs.set_question_asker_lm(outline_lm)
    lm_configs.set_outline_gen_lm(outline_lm)
    lm_configs.set_article_gen_lm(article_lm)
    lm_configs.set_article_polish_lm(article_lm)

    return lm_configs


def build_runner_args() -> STORMWikiRunnerArguments:
    """Build STORM runner arguments from environment variables."""
    from knowledge_storm import STORMWikiRunnerArguments

    return STORMWikiRunnerArguments(
        output_dir=os.environ.get("STORM_OUTPUT_DIR", "/tmp/storm_output"),
        search_top_k=int(os.environ.get("STORM_SEARCH_TOP_K", "3")),
        max_perspective=int(os.environ.get("STORM_MAX_PERSPECTIVES", "2")),
        max_conv_turn=int(os.environ.get("STORM_MAX_CONV_TURNS", "2")),
        max_thread_num=int(os.environ.get("STORM_MAX_THREAD_NUM", "3")),
    )


def _parse_summary(
    summary: dict[str, Any],
    request_id: str,
) -> GenerateReportResponse:
    """Parse runner.summary() output into our response model.

    M2 parses the title, sections, and sentences.  Citation translation
    is deferred to M3 (SPEC-DEEP-001 plan.md line 94).
    """
    title = summary.get("title", "")
    raw_sections = summary.get("sections", [])
    sections: list[Section] = []

    for raw_section in raw_sections:
        sentences: list[Sentence] = []
        for raw_sentence in raw_section.get("sentences", []):
            text = raw_sentence.get("text", "")
            sentences.append(Sentence(text=text))
        sections.append(
            Section(
                heading=raw_section.get("heading", ""),
                level=raw_section.get("level", 1),
                sentences=sentences,
            )
        )

    return GenerateReportResponse(
        request_id=request_id,
        title=title,
        sections=sections,
        citations=[],  # Citation translation deferred to M3
        schema_version=1,
    )


def run(req: GenerateReportRequest) -> GenerateReportResponse:
    """Execute the full STORM pipeline for a given request.

    # @MX:WARN: [AUTO] LLM trust boundary — all LM calls through LiteLLM proxy.
    # @MX:REASON: Gateway config must point at LiteLLM; no direct vendor access.
    """
    from knowledge_storm import STORMWikiRunner

    from storm.gateway import GatewayConfig
    from storm.inject_rm import InjectedRM

    with Timer() as timer:
        gateway = GatewayConfig()
        lm_configs = build_lm_configs(gateway)
        args = build_runner_args()

        rm = InjectedRM(
            docs=req.docs,
            top_k=int(os.environ.get("STORM_SEARCH_TOP_K", "3")),
        )

        runner = STORMWikiRunner(args, lm_configs, rm)
        runner.run(
            topic=req.query,
            do_research=True,
            do_generate_article=True,
        )

        summary = runner.summary()

    logger.info(
        {
            "message": "pipeline completed",
            "request_id": req.request_id,
            "elapsed_ms": timer.elapsed_ms,
            "sections_count": len(summary.get("sections", [])),
        }
    )

    return _parse_summary(summary, req.request_id)
