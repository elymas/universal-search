package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RenderOpts carries optional metadata rendered alongside the report.
type RenderOpts struct {
	CommitSHA  string
	Branch     string
	JudgeModel string
	CostUSD    float64
	RuntimeS   float64
}

// RenderMarkdown produces the operator-facing markdown report for a run.
// REQ-EVAL1-007: includes a Lowest-Scoring Queries section with judge
// rationales, a per-category breakdown, null-query listing, and cost.
func RenderMarkdown(rep Report, opts RenderOpts) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# SPEC-EVAL-001 Citation Faithfulness Report\n\n")
	scored := len(rep.Queries) - rep.NullCount - rep.OverrideCount
	fmt.Fprintf(&b, "- Mean score: **%.3f** (over %d scored queries)\n", rep.MeanScore, scored)
	fmt.Fprintf(&b, "- Null (judge unavailable): %d\n", rep.NullCount)
	fmt.Fprintf(&b, "- Overrides applied: %d\n", rep.OverrideCount)
	if opts.JudgeModel != "" {
		fmt.Fprintf(&b, "- Judge model: %s\n", opts.JudgeModel)
	}
	if opts.CommitSHA != "" {
		fmt.Fprintf(&b, "- Commit: %s (branch %s)\n", opts.CommitSHA, opts.Branch)
	}
	fmt.Fprintf(&b, "- LLM judge cost: $%.2f\n", opts.CostUSD)
	if opts.RuntimeS > 0 {
		fmt.Fprintf(&b, "- Runtime: %.1fs\n", opts.RuntimeS)
	}
	b.WriteString("\n")

	renderCategoryBreakdown(&b, rep)
	renderLowestScoring(&b, rep)
	renderNullQueries(&b, rep)

	return b.String()
}

func renderCategoryBreakdown(b *strings.Builder, rep Report) {
	type agg struct {
		sum   float64
		count int
	}
	cats := map[string]*agg{}
	var order []string
	for _, q := range rep.Queries {
		if q.Score == nil || q.Overridden {
			continue
		}
		a, ok := cats[q.Category]
		if !ok {
			a = &agg{}
			cats[q.Category] = a
			order = append(order, q.Category)
		}
		a.sum += *q.Score
		a.count++
	}
	sort.Strings(order)
	b.WriteString("## Per-Category Breakdown\n\n")
	b.WriteString("| Category | Mean | Count |\n|---|---|---|\n")
	for _, c := range order {
		a := cats[c]
		mean := 0.0
		if a.count > 0 {
			mean = a.sum / float64(a.count)
		}
		fmt.Fprintf(b, "| %s | %.3f | %d |\n", c, mean, a.count)
	}
	b.WriteString("\n")
}

func renderLowestScoring(b *strings.Builder, rep Report) {
	scored := make([]QueryResult, 0, len(rep.Queries))
	for _, q := range rep.Queries {
		if q.Score != nil && !q.Overridden {
			scored = append(scored, q)
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return *scored[i].Score < *scored[j].Score
	})
	limit := 10
	if len(scored) < limit {
		limit = len(scored)
	}

	b.WriteString("## Lowest-Scoring Queries\n\n")
	if limit == 0 {
		b.WriteString("_No scored queries._\n\n")
		return
	}
	for _, q := range scored[:limit] {
		fmt.Fprintf(b, "### %s (%s, %s) — score %.3f\n\n", q.QueryID, q.Category, q.Locale, *q.Score)
		for _, c := range q.PerClaim {
			if c.Supported {
				continue
			}
			fmt.Fprintf(b, "- UNSUPPORTED: %q\n  - rationale: %s\n", c.Text, c.JudgeRationale)
		}
		b.WriteString("\n")
	}
}

func renderNullQueries(b *strings.Builder, rep Report) {
	var nulls []QueryResult
	for _, q := range rep.Queries {
		if q.Score == nil && !q.Overridden {
			nulls = append(nulls, q)
		}
	}
	if len(nulls) == 0 {
		return
	}
	b.WriteString("## Null (Unscoreable) Queries\n\n")
	for _, q := range nulls {
		fmt.Fprintf(b, "- %s (%s): %s\n", q.QueryID, q.Locale, q.ErrorClass)
	}
	b.WriteString("\n")
}

// WriteLatest writes the rendered markdown to .moai/eval/reports/latest.md.
// REQ-EVAL1-007. (JSON history writer is deferred to V1.1 per REQ-EVAL1-010.)
func WriteLatest(dir string, rep Report, opts RenderOpts) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("report: mkdir: %w", err)
	}
	md := RenderMarkdown(rep, opts)
	p := filepath.Join(dir, "latest.md")
	if err := os.WriteFile(p, []byte(md), 0o644); err != nil { //nolint:gosec // operator-readable report.
		return fmt.Errorf("report: write latest: %w", err)
	}
	return nil
}
