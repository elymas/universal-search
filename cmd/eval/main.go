// Command eval runs the SPEC-EVAL-001 citation faithfulness benchmark.
//
// It loads the frozen golden set + corpus, drives the synthesis path for each
// query, scores responses with the DeepEval judge bridge, writes a markdown
// report, and exits with the release-gate code (0/1/2/3 per REQ-EVAL1-008).
//
// Environment:
//   - RESEARCHER_BASE_URL:   synthesis sidecar base URL (default localhost:8081)
//   - EVAL_JUDGE_BASE_URL:   DeepEval judge base URL (default same as researcher)
//   - EVAL_JUDGE_MODEL:      LiteLLM judge model id (default claude-haiku-4-5)
//   - EVAL_CONCURRENCY:      max parallel queries (default 5, NFR-EVAL1-004)
//   - EVAL_REPORTS_DIR:      where latest.md is written (default .moai/eval/reports)
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/elymas/universal-search/internal/eval"
	"github.com/elymas/universal-search/internal/eval/ci"
	"github.com/elymas/universal-search/internal/eval/golden"
	"github.com/elymas/universal-search/internal/eval/runner"
	"github.com/elymas/universal-search/internal/eval/scorer"
	"github.com/elymas/universal-search/internal/obs/metrics"
	"github.com/elymas/universal-search/internal/synthesis"
	"github.com/elymas/universal-search/pkg/types"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr))
}

// run wires the benchmark pipeline and returns the gate exit code. Separated
// from main for testability.
func run(stdout, stderr io.Writer) int {
	ctx := context.Background()

	queries, err := golden.LoadQueries(golden.QueriesPath())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: load queries: %v\n", err)
		return ci.ExitMalformed
	}
	corpusDocs, err := golden.LoadCorpus(golden.CorpusDir())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: load corpus: %v\n", err)
		return ci.ExitMalformed
	}
	overrides, err := golden.LoadOverrides(golden.OverridesPath())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: load overrides: %v\n", err)
		return ci.ExitMalformed
	}
	if err := golden.CheckOverrideCap(overrides, 5); err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: %v\n", err)
		_, _ = fmt.Fprintln(stdout, "EVAL-001 result=FAIL reason=override_cap")
		return ci.ExitBelow
	}

	// corpus map: doc_id -> body text (judge input).
	corpusBody := make(map[string]string, len(corpusDocs))
	for id, d := range corpusDocs {
		corpusBody[id] = d.Body
	}

	synthCfg, err := synthesis.LoadConfig()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: synthesis config: %v\n", err)
		return ci.ExitMalformed
	}
	synthClient, err := synthesis.New(synthCfg, nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: synthesis client: %v\n", err)
		return ci.ExitMalformed
	}

	judgeURL := os.Getenv("EVAL_JUDGE_BASE_URL")
	if judgeURL == "" {
		judgeURL = synthCfg.BaseURL
	}
	bridge := scorer.NewBridge(judgeURL, 30*time.Second)

	cfg := runner.Config{Concurrency: concurrency()}
	synthAdapter := corpusSynthesizer{client: synthClient, corpus: corpusDocs}

	start := time.Now()
	rep, err := runner.Run(ctx, cfg, queries, corpusBody, overrides, synthAdapter, bridge)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "eval: run: %v\n", err)
		return ci.ExitMalformed
	}

	opts := runner.RenderOpts{
		JudgeModel: judgeModel(),
		Branch:     os.Getenv("GITHUB_REF_NAME"),
		CommitSHA:  os.Getenv("GITHUB_SHA"),
		RuntimeS:   time.Since(start).Seconds(),
	}
	if dir := reportsDir(); dir != "" {
		if werr := runner.WriteLatest(dir, rep, opts); werr != nil {
			_, _ = fmt.Fprintf(stderr, "eval: write report: %v\n", werr)
		}
	}

	res := ci.Decide(eval.ToQueryScores(rep), overrideIDs(overrides), ci.Thresholds{Mean: 0.85, Floor: 0.50, OverrideCap: 5})

	// SPEC-EVAL-001 observability: record the run outcome + score. Emitted to a
	// fresh registry; a long-running host scrapes these families, while the
	// one-shot CI run exposes them via the report summary line below.
	reg := metrics.NewRegistry()
	reg.EvalRuns.WithLabelValues(metricOutcome(res)).Inc()
	reg.EvalScore.Set(res.Mean)

	_, _ = fmt.Fprintln(stdout, ci.SummaryLine(res))
	if res.Reason != "" {
		_, _ = fmt.Fprintf(stdout, "EVAL-001 reason=%q\n", res.Reason)
	}
	return res.ExitCode
}

// corpusSynthesizer drives the real synthesis path against the frozen corpus.
// It selects the docs each query expects (or all corpus docs when none are
// declared) and feeds them to the synthesis client.
type corpusSynthesizer struct {
	client *synthesis.Client
	corpus map[string]types.NormalizedDoc
}

func (s corpusSynthesizer) Synthesize(ctx context.Context, q golden.Query, _ map[string]string) (synthesis.Result, error) {
	docs := make([]types.NormalizedDoc, 0, len(q.ExpectedSources))
	for _, id := range q.ExpectedSources {
		if d, ok := s.corpus[id]; ok {
			docs = append(docs, d)
		}
	}
	if len(docs) == 0 {
		// Edge queries with no expected sources: feed the full corpus.
		for _, d := range s.corpus {
			docs = append(docs, d)
		}
	}
	lang := q.Locale
	return s.client.Synthesize(ctx, q.Query, lang, docs)
}

func overrideIDs(overrides []golden.Override) []string {
	out := make([]string, 0, len(overrides))
	for _, o := range overrides {
		out = append(out, o.QueryID)
	}
	return out
}

func concurrency() int {
	if v := os.Getenv("EVAL_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 5
}

func judgeModel() string {
	if v := os.Getenv("EVAL_JUDGE_MODEL"); v != "" {
		return v
	}
	return "claude-haiku-4-5"
}

// metricOutcome maps a gate result to the eval metric `outcome` label value
// (pass|fail|null), reusing the existing OBS-001 allowlist label.
func metricOutcome(res ci.Result) string {
	switch {
	case res.Pass:
		return "pass"
	case res.NullCount > 0:
		return "null"
	default:
		return "fail"
	}
}

func reportsDir() string {
	if v := os.Getenv("EVAL_REPORTS_DIR"); v != "" {
		return v
	}
	return ".moai/eval/reports"
}
