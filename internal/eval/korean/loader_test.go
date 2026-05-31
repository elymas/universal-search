package korean

import (
	"fmt"
	"strings"
	"testing"
)

// validQuery returns a syntactically valid golden-set JSON line with the
// given id and category, suitable for assembling a well-formed 50-line set.
func validQuery(id string, cat Category) string {
	lang := "ko"
	router := "korean"
	naverRel := "true"
	vertical := `,"expected_naver_vertical":"blog"`
	sources := `["naver"]`
	switch cat {
	case CategoryCodeMixed:
		lang, router = "mixed", "mixed"
	case CategoryAcademicTech:
		naverRel = "false"
		vertical = ""
		sources = `["arxiv","github"]`
	}
	return fmt.Sprintf(
		`{"query_id":%q,"query_text":"테스트 쿼리","category":%q,"expected_lang":%q,"expected_router_class":%q,"expected_naver_relevant":%s%s,"expected_sources":%s}`,
		id, cat, lang, router, naverRel, vertical, sources,
	)
}

// fullValidSet builds a 50-line golden set honoring the 12/10/8/8/6/6
// distribution with unique KR-NNN ids.
func fullValidSet() string {
	order := []Category{
		CategoryNews, CategoryBlog, CategoryShopping,
		CategoryAcademicTech, CategoryCodeMixed, CategoryCultural,
	}
	var lines []string
	n := 1
	for _, cat := range order {
		for i := 0; i < ExpectedCategoryDistribution[cat]; i++ {
			id := fmt.Sprintf("KR-%03d", n)
			line := validQuery(id, cat)
			if cat == CategoryCultural {
				// cultural targets naver but as a generic ko query
				line = fmt.Sprintf(
					`{"query_id":%q,"query_text":"문화 쿼리","category":"cultural","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":true,"expected_sources":["naver","koreanews"]}`,
					id,
				)
			}
			lines = append(lines, line)
			n++
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestLoadGoldenSet_50Objects(t *testing.T) {
	t.Parallel()
	queries, err := LoadGoldenSet(strings.NewReader(fullValidSet()))
	if err != nil {
		t.Fatalf("LoadGoldenSet: unexpected error: %v", err)
	}
	if len(queries) != GoldenSetSize {
		t.Fatalf("got %d queries, want %d", len(queries), GoldenSetSize)
	}
}

func TestLoadGoldenSet_CategoryDistribution(t *testing.T) {
	t.Parallel()
	queries, err := LoadGoldenSet(strings.NewReader(fullValidSet()))
	if err != nil {
		t.Fatalf("LoadGoldenSet: %v", err)
	}
	counts := map[Category]int{}
	for _, q := range queries {
		counts[q.Category]++
	}
	for cat, want := range ExpectedCategoryDistribution {
		if counts[cat] != want {
			t.Errorf("category %q: got %d, want %d", cat, counts[cat], want)
		}
	}
}

func TestLoadGoldenSet_AllRequiredFields(t *testing.T) {
	t.Parallel()
	queries, err := LoadGoldenSet(strings.NewReader(fullValidSet()))
	if err != nil {
		t.Fatalf("LoadGoldenSet: %v", err)
	}
	for _, q := range queries {
		if q.QueryID == "" || q.QueryText == "" || q.Category == "" ||
			q.ExpectedLang == "" || q.ExpectedRouterClass == "" || len(q.ExpectedSources) == 0 {
			t.Errorf("query %q missing required field: %+v", q.QueryID, q)
		}
	}
}

func TestLoadGoldenSet_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := LoadGoldenSet(strings.NewReader("{not valid json}\n"))
	var se *SchemaError
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !asSchemaError(err, &se) {
		t.Fatalf("expected *SchemaError, got %T: %v", err, err)
	}
}

func TestLoadGoldenSet_RejectsPhantomSourceID(t *testing.T) {
	t.Parallel()
	phantoms := []string{
		"naver-news", "naver-blog", "naver-shopping",
		"naver-academic", "daum-news", "korea-news-crawler",
	}
	for _, phantom := range phantoms {
		phantom := phantom
		t.Run(phantom, func(t *testing.T) {
			t.Parallel()
			line := fmt.Sprintf(
				`{"query_id":"KR-001","query_text":"q","category":"news","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":true,"expected_sources":[%q]}`,
				phantom,
			)
			_, err := LoadGoldenSet(strings.NewReader(line + "\n"))
			if err == nil {
				t.Fatalf("phantom SourceID %q was accepted; want rejection", phantom)
			}
			if !strings.Contains(err.Error(), phantom) {
				t.Errorf("error should name the phantom ID %q: %v", phantom, err)
			}
		})
	}
}

func TestLoadGoldenSet_RejectsBadVertical(t *testing.T) {
	t.Parallel()
	line := `{"query_id":"KR-001","query_text":"q","category":"news","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":true,"expected_naver_vertical":"academic","expected_sources":["naver"]}`
	_, err := LoadGoldenSet(strings.NewReader(line + "\n"))
	if err == nil {
		t.Fatal("expected rejection of academic vertical, got nil")
	}
}

func TestLoadGoldenSet_WrongCount(t *testing.T) {
	t.Parallel()
	one := validQuery("KR-001", CategoryNews) + "\n"
	_, err := LoadGoldenSet(strings.NewReader(one))
	if err == nil {
		t.Fatal("expected error for wrong object count, got nil")
	}
	if !strings.Contains(err.Error(), "50") {
		t.Errorf("error should mention expected count 50: %v", err)
	}
}

func TestLoadGoldenSet_WrongDistribution(t *testing.T) {
	t.Parallel()
	// 50 objects but all news → distribution violation.
	var lines []string
	for i := 1; i <= 50; i++ {
		lines = append(lines, validQuery(fmt.Sprintf("KR-%03d", i), CategoryNews))
	}
	_, err := LoadGoldenSet(strings.NewReader(strings.Join(lines, "\n") + "\n"))
	if err == nil {
		t.Fatal("expected distribution error, got nil")
	}
}

func TestLoadGoldenSet_DuplicateID(t *testing.T) {
	t.Parallel()
	set := fullValidSet()
	// Corrupt: replace KR-002 with KR-001 to force a duplicate.
	set = strings.Replace(set, `"query_id":"KR-002"`, `"query_id":"KR-001"`, 1)
	_, err := LoadGoldenSet(strings.NewReader(set))
	if err == nil {
		t.Fatal("expected duplicate id error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error should mention duplicate: %v", err)
	}
}

func TestLoadGoldenSet_BadQueryIDFormat(t *testing.T) {
	t.Parallel()
	line := `{"query_id":"KR-1","query_text":"q","category":"news","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":true,"expected_sources":["naver"]}`
	_, err := LoadGoldenSet(strings.NewReader(line + "\n"))
	if err == nil {
		t.Fatal("expected query_id format error, got nil")
	}
}

func TestLoadGoldenSet_NotesTooLong(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("가", 201)
	line := `{"query_id":"KR-001","query_text":"q","category":"news","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":true,"expected_sources":["naver"],"notes":"` + long + `"}`
	_, err := LoadGoldenSet(strings.NewReader(line + "\n"))
	if err == nil || !strings.Contains(err.Error(), "notes") {
		t.Fatalf("expected notes-length error, got %v", err)
	}
}

func TestLoadGoldenSet_CodeMixedRequiresMixed(t *testing.T) {
	t.Parallel()
	// code-mixed category but declared ko/korean → must be rejected.
	line := `{"query_id":"KR-001","query_text":"q","category":"code-mixed","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":true,"expected_sources":["naver"]}`
	_, err := LoadGoldenSet(strings.NewReader(line + "\n"))
	if err == nil || !strings.Contains(err.Error(), "mixed") {
		t.Fatalf("expected code-mixed/mixed error, got %v", err)
	}
}

func TestLoadGoldenSet_VerticalWithoutRelevance(t *testing.T) {
	t.Parallel()
	line := `{"query_id":"KR-001","query_text":"q","category":"news","expected_lang":"ko","expected_router_class":"korean","expected_naver_relevant":false,"expected_naver_vertical":"blog","expected_sources":["naver"]}`
	_, err := LoadGoldenSet(strings.NewReader(line + "\n"))
	if err == nil {
		t.Fatal("vertical set with relevant=false should be rejected")
	}
}

func TestOrderedCategories_StableAndComplete(t *testing.T) {
	t.Parallel()
	cats := OrderedCategories()
	if len(cats) != len(ExpectedCategoryDistribution) {
		t.Fatalf("OrderedCategories len = %d, want %d", len(cats), len(ExpectedCategoryDistribution))
	}
	for i := 1; i < len(cats); i++ {
		if cats[i-1] > cats[i] {
			t.Errorf("not sorted at %d: %q > %q", i, cats[i-1], cats[i])
		}
	}
}

// asSchemaError is a tiny errors.As helper kept local to avoid an import in
// every call site.
func asSchemaError(err error, target **SchemaError) bool {
	se, ok := err.(*SchemaError)
	if ok {
		*target = se
	}
	return ok
}
