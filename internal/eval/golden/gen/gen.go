//go:build ignore

// Command gen authors the SPEC-EVAL-001 frozen golden set: queries.jsonl,
// the corpus/*.json NormalizedDoc fixtures, manifest.json, and overrides.json.
//
// Provenance: all fixtures are hand-authored synthetic content created for the
// benchmark. No PII, no scraped production data, no real user queries.
// Run once: `go run ./internal/eval/golden/gen` from the repo root.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type query struct {
	ID              string   `json:"id"`
	Query           string   `json:"query"`
	Locale          string   `json:"locale"`
	ExpectedSources []string `json:"expected_sources,omitempty"`
	Category        string   `json:"category"`
	Notes           string   `json:"notes,omitempty"`
}

type doc struct {
	ID          string         `json:"id"`
	SourceID    string         `json:"source_id"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Snippet     string         `json:"snippet"`
	PublishedAt time.Time      `json:"published_at"`
	RetrievedAt time.Time      `json:"retrieved_at"`
	Author      string         `json:"author"`
	Score       float64        `json:"score"`
	Lang        string         `json:"lang"`
	DocType     string         `json:"doc_type"`
	Citations   []string       `json:"citations,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Hash        string         `json:"hash"`
}

func mustWrite(path string, v any) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	root := "internal/eval/golden"
	corpusDir := filepath.Join(root, "corpus")
	if err := os.MkdirAll(corpusDir, 0o755); err != nil {
		panic(err)
	}
	retrieved := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	published := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	// Topic seeds: (slug, EN title, EN body, KO title, KO body).
	type topic struct {
		slug          string
		enT, enB      string
		koT, koB      string
		source, dtype string
	}
	topics := []topic{
		{"quantum", "Quantum supremacy milestone 2019", "Google reported a quantum processor completing a sampling task in 200 seconds that would take a classical supercomputer about 10000 years. The claim was later contested by IBM researchers who argued a classical method could finish it in days. The result is widely cited as the first quantum supremacy demonstration.", "2019년 양자 우월성 이정표", "구글은 양자 프로세서가 200초 만에 표본 추출 작업을 완료했다고 발표했다. 고전 슈퍼컴퓨터로는 약 1만 년이 걸린다고 주장했다. 이 결과는 최초의 양자 우월성 시연으로 널리 인용된다.", "arxiv", "paper"},
		{"transformer", "Attention is all you need", "The transformer architecture replaced recurrence with self-attention, enabling parallel training over sequences. It became the foundation for large language models. The original paper introduced multi-head attention and positional encodings.", "어텐션이 전부다", "트랜스포머 구조는 순환을 자기어텐션으로 대체하여 시퀀스를 병렬로 학습할 수 있게 했다. 이는 대규모 언어 모델의 기초가 되었다. 원 논문은 멀티헤드 어텐션과 위치 인코딩을 도입했다.", "arxiv", "paper"},
		{"rustgc", "Rust memory model without GC", "Rust achieves memory safety without a garbage collector using an ownership and borrowing system checked at compile time. Each value has a single owner and references are validated by the borrow checker. This eliminates entire classes of memory bugs.", "GC 없는 러스트 메모리 모델", "러스트는 컴파일 시점에 검사하는 소유권과 빌림 시스템으로 가비지 컬렉터 없이 메모리 안전성을 달성한다. 각 값은 단일 소유자를 가지며 참조는 빌림 검사기가 검증한다. 이는 전체 메모리 버그 부류를 제거한다.", "github", "repo"},
		{"k8s", "Kubernetes scheduling basics", "The Kubernetes scheduler assigns pods to nodes based on resource requests, affinity rules, and taints. It runs a filtering phase followed by a scoring phase. The highest scoring feasible node wins the pod.", "쿠버네티스 스케줄링 기초", "쿠버네티스 스케줄러는 리소스 요청, 어피니티 규칙, 테인트를 기반으로 파드를 노드에 할당한다. 필터링 단계 후 점수 매기기 단계를 실행한다. 가장 높은 점수의 실행 가능한 노드가 파드를 차지한다.", "github", "repo"},
		{"crispr", "CRISPR gene editing overview", "CRISPR-Cas9 uses a guide RNA to direct the Cas9 nuclease to a specific DNA sequence where it makes a double-strand break. The cell's repair machinery then introduces edits. It has transformed genetic research since 2012.", "크리스퍼 유전자 편집 개요", "크리스퍼-Cas9는 가이드 RNA를 사용하여 Cas9 핵산분해효소를 특정 DNA 서열로 유도해 이중 가닥 절단을 만든다. 세포의 복구 기구가 편집을 도입한다. 2012년 이후 유전 연구를 변혁했다.", "arxiv", "paper"},
		{"raft", "Raft consensus algorithm", "Raft is a consensus algorithm designed to be understandable. It elects a leader that manages a replicated log, and followers replicate entries from the leader. Safety is guaranteed through term numbers and log matching.", "래프트 합의 알고리즘", "래프트는 이해하기 쉽도록 설계된 합의 알고리즘이다. 복제 로그를 관리하는 리더를 선출하고 팔로워는 리더로부터 항목을 복제한다. 안전성은 임기 번호와 로그 매칭으로 보장된다.", "arxiv", "paper"},
		{"httpcache", "HTTP caching semantics", "HTTP caching uses Cache-Control directives such as max-age and no-store to control freshness. Conditional requests with ETag and If-None-Match enable revalidation. A 304 Not Modified response saves bandwidth.", "HTTP 캐싱 시맨틱", "HTTP 캐싱은 max-age와 no-store 같은 Cache-Control 지시어로 신선도를 제어한다. ETag와 If-None-Match를 사용한 조건부 요청으로 재검증이 가능하다. 304 Not Modified 응답은 대역폭을 절약한다.", "rss", "article"},
		{"vectordb", "Vector databases for retrieval", "Vector databases store high-dimensional embeddings and support approximate nearest neighbour search. Common index types include HNSW and IVF. They power semantic retrieval in modern search systems.", "검색을 위한 벡터 데이터베이스", "벡터 데이터베이스는 고차원 임베딩을 저장하고 근사 최근접 이웃 검색을 지원한다. 일반적인 인덱스 유형으로 HNSW와 IVF가 있다. 현대 검색 시스템의 의미 검색을 구동한다.", "github", "repo"},
		{"tls13", "TLS 1.3 handshake", "TLS 1.3 reduces the handshake to one round trip by removing legacy cipher suites and key exchange methods. It mandates forward secrecy and encrypts more of the handshake. Zero round-trip resumption is supported with caveats.", "TLS 1.3 핸드셰이크", "TLS 1.3은 레거시 암호 스위트와 키 교환 방식을 제거하여 핸드셰이크를 한 번의 왕복으로 줄인다. 순방향 비밀성을 의무화하고 핸드셰이크의 더 많은 부분을 암호화한다. 제로 왕복 재개를 지원한다.", "rss", "article"},
		{"wasm", "WebAssembly runtime model", "WebAssembly is a portable bytecode that runs in a sandboxed stack-based virtual machine. It offers near-native performance and language independence. Browsers and standalone runtimes both execute it.", "웹어셈블리 런타임 모델", "웹어셈블리는 샌드박스화된 스택 기반 가상 머신에서 실행되는 이식 가능한 바이트코드다. 거의 네이티브에 가까운 성능과 언어 독립성을 제공한다. 브라우저와 독립 런타임 모두 이를 실행한다.", "github", "repo"},
		{"diffusion", "Diffusion models for images", "Diffusion models generate images by reversing a gradual noising process. A neural network learns to denoise samples step by step. They have surpassed GANs on many image generation benchmarks.", "이미지를 위한 확산 모델", "확산 모델은 점진적인 노이즈 추가 과정을 역전시켜 이미지를 생성한다. 신경망이 단계별로 샘플의 노이즈를 제거하는 법을 학습한다. 많은 이미지 생성 벤치마크에서 GAN을 능가했다.", "arxiv", "paper"},
		{"sqlindex", "SQL index internals", "A B-tree index speeds up lookups by keeping keys sorted on disk pages. The query planner chooses an index when selectivity is high. Composite indexes follow the leftmost-prefix rule.", "SQL 인덱스 내부", "B-트리 인덱스는 디스크 페이지에 키를 정렬해 두어 조회 속도를 높인다. 쿼리 플래너는 선택도가 높을 때 인덱스를 선택한다. 복합 인덱스는 최좌측 접두사 규칙을 따른다.", "rss", "article"},
		{"goroutine", "Go goroutine scheduling", "Go multiplexes goroutines onto OS threads using a work-stealing scheduler. The GOMAXPROCS setting bounds parallelism. Channels provide synchronisation between goroutines.", "Go 고루틴 스케줄링", "Go는 작업 훔치기 스케줄러를 사용하여 고루틴을 OS 스레드에 다중화한다. GOMAXPROCS 설정이 병렬성을 제한한다. 채널은 고루틴 간 동기화를 제공한다.", "github", "repo"},
		{"oauth", "OAuth 2.0 authorization code flow", "The authorization code flow exchanges a short-lived code for an access token at the token endpoint. PKCE protects public clients from interception. Refresh tokens enable long-lived sessions.", "OAuth 2.0 인가 코드 흐름", "인가 코드 흐름은 토큰 엔드포인트에서 단명 코드를 액세스 토큰으로 교환한다. PKCE는 공개 클라이언트를 가로채기로부터 보호한다. 갱신 토큰은 장기 세션을 가능하게 한다.", "rss", "article"},
		{"llmquant", "LLM quantization tradeoffs", "Quantization reduces model weights to lower precision such as int8 or int4, shrinking memory and speeding inference. It can degrade accuracy, mitigated by techniques like GPTQ and AWQ. The tradeoff depends on the task.", "LLM 양자화 트레이드오프", "양자화는 모델 가중치를 int8이나 int4 같은 낮은 정밀도로 줄여 메모리를 축소하고 추론을 가속한다. 정확도를 떨어뜨릴 수 있으나 GPTQ와 AWQ 같은 기법으로 완화한다. 트레이드오프는 작업에 따라 다르다.", "arxiv", "paper"},
	}

	docs := make([]doc, 0, len(topics)*4)
	docByTopic := map[string][]string{} // slug -> list of doc IDs (en1,en2,ko1,ko2)
	n := 0
	mk := func(t topic, suffix, lang, title, body string) string {
		n++
		id := fmt.Sprintf("doc-%03d", n)
		src := t.source
		d := doc{
			ID: id, SourceID: src,
			URL:         fmt.Sprintf("https://example.test/%s/%s-%s", src, t.slug, suffix),
			Title:       title,
			Body:        body,
			Snippet:     title,
			PublishedAt: published,
			RetrievedAt: retrieved,
			Author:      "synthetic-author",
			Score:       0.8,
			Lang:        lang,
			DocType:     t.dtype,
			Metadata:    map[string]any{"provenance": "synthetic", "pii": false, "spec": "SPEC-EVAL-001"},
			Hash:        "",
		}
		docs = append(docs, d)
		docByTopic[t.slug] = append(docByTopic[t.slug], id)
		return id
	}
	for _, t := range topics {
		mk(t, "a", "en", t.enT, t.enB)
		mk(t, "b", "en", t.enT+" (deep dive)", t.enB+" Subsequent work expanded on these findings with broader experiments.")
		mk(t, "ko-a", "ko", t.koT, t.koB)
		mk(t, "ko-b", "ko", t.koT+" (심화)", t.koB+" 후속 연구는 더 광범위한 실험으로 이러한 발견을 확장했다.")
	}
	// docs now has 14*4 = 56 fixtures.

	// Write corpus fixtures.
	for _, d := range docs {
		mustWrite(filepath.Join(corpusDir, d.ID+".json"), d)
	}

	// Build queries: 35 EN + 15 KO. Each references expected_sources in corpus.
	enCats := []string{"factual", "comparison", "synthesis", "edge"}
	queries := make([]query, 0, 50)
	qn := 0
	addQ := func(text, locale, cat string, sources ...string) {
		qn++
		queries = append(queries, query{
			ID:              fmt.Sprintf("EVAL-001-Q%03d", qn),
			Query:           text,
			Locale:          locale,
			ExpectedSources: sources,
			Category:        cat,
			Notes:           "synthetic golden query",
		})
	}
	// 35 EN queries across the 14 topics (cycle, 2-3 per topic).
	enTexts := []string{
		"What was the quantum supremacy claim in 2019?",
		"How did IBM respond to the quantum supremacy result?",
		"What does the transformer architecture replace?",
		"Why are transformers good for large language models?",
		"How does Rust achieve memory safety without a garbage collector?",
		"What is the borrow checker in Rust?",
		"How does the Kubernetes scheduler assign pods?",
		"What phases does Kubernetes scheduling use?",
		"How does CRISPR-Cas9 edit DNA?",
		"What role does guide RNA play in CRISPR?",
		"What problem does the Raft algorithm solve?",
		"How does Raft guarantee safety?",
		"What HTTP directives control cache freshness?",
		"How do conditional HTTP requests save bandwidth?",
		"What index types do vector databases use?",
		"How do vector databases power semantic retrieval?",
		"How many round trips does the TLS 1.3 handshake need?",
		"What does TLS 1.3 mandate for security?",
		"What virtual machine model does WebAssembly use?",
		"What performance does WebAssembly offer?",
		"How do diffusion models generate images?",
		"How do diffusion models compare to GANs?",
		"How does a B-tree SQL index speed up lookups?",
		"What is the leftmost-prefix rule for composite indexes?",
		"How does Go schedule goroutines?",
		"What does GOMAXPROCS control in Go?",
		"What does the OAuth authorization code flow exchange?",
		"How does PKCE protect OAuth public clients?",
		"What does LLM quantization reduce?",
		"What techniques mitigate quantization accuracy loss?",
		"Compare quantum supremacy claims by Google and IBM.",
		"Summarize how transformers and diffusion models differ.",
		"What are the tradeoffs of int4 quantization for LLMs?",
		"Explain the relationship between Raft terms and log matching.",
		"Does an empty query return a graceful result?",
	}
	for i, txt := range enTexts {
		topicIdx := (i / 2) % len(topics)
		cat := enCats[i%len(enCats)]
		if i >= 30 {
			cat = "synthesis"
		}
		if i == len(enTexts)-1 {
			cat = "edge"
		}
		t := topics[topicIdx]
		ids := docByTopic[t.slug]
		addQ(txt, "en", cat, ids[0], ids[1]) // two EN sources
	}
	// 15 KO queries.
	koTexts := []string{
		"2019년 양자 우월성 주장은 무엇이었나?",
		"트랜스포머 구조는 무엇을 대체하는가?",
		"러스트는 어떻게 GC 없이 메모리 안전성을 달성하는가?",
		"쿠버네티스 스케줄러는 어떻게 파드를 할당하는가?",
		"크리스퍼-Cas9는 어떻게 DNA를 편집하는가?",
		"래프트 알고리즘은 어떤 문제를 해결하는가?",
		"어떤 HTTP 지시어가 캐시 신선도를 제어하는가?",
		"벡터 데이터베이스는 어떤 인덱스 유형을 사용하는가?",
		"TLS 1.3 핸드셰이크는 몇 번의 왕복이 필요한가?",
		"웹어셈블리는 어떤 가상 머신 모델을 사용하는가?",
		"확산 모델은 어떻게 이미지를 생성하는가?",
		"B-트리 인덱스는 어떻게 조회 속도를 높이는가?",
		"Go는 어떻게 고루틴을 스케줄링하는가?",
		"OAuth 인가 코드 흐름은 무엇을 교환하는가?",
		"LLM 양자화는 무엇을 줄이는가?",
	}
	for i, txt := range koTexts {
		t := topics[i%len(topics)]
		ids := docByTopic[t.slug]
		addQ(txt, "ko", "korean", ids[2], ids[3]) // two KO sources
	}

	// Write queries.jsonl (one compact JSON object per line).
	var b []byte
	for _, q := range queries {
		line, err := json.Marshal(q)
		if err != nil {
			panic(err)
		}
		b = append(b, line...)
		b = append(b, '\n')
	}
	if err := os.WriteFile(filepath.Join(root, "queries.jsonl"), b, 0o644); err != nil {
		panic(err)
	}

	// Empty overrides list.
	if err := os.WriteFile(filepath.Join(root, "overrides.json"), []byte("[]\n"), 0o644); err != nil {
		panic(err)
	}

	// Manifest with corpus revision pin.
	manifest := map[string]any{
		"corpus_revision": 1,
		"doc_count":       len(docs),
		"query_count":     len(queries),
		"en_count":        35,
		"ko_count":        15,
		"provenance":      "synthetic; no PII; authored for SPEC-EVAL-001 benchmark",
		"generated_at":    retrieved.Format(time.RFC3339),
	}
	mustWrite(filepath.Join(root, "manifest.json"), manifest)

	fmt.Printf("wrote %d corpus docs, %d queries\n", len(docs), len(queries))
}
