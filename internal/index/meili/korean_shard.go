package meili

// KoreanIndexName is the Meilisearch index name for Korean-language documents.
// SPEC-IDX-003 REQ-IDX-003-011: dual-index architecture with "usearch" (default)
// and "usearch-ko" (Korean shard).
const KoreanIndexName = "usearch-ko"

// KoreanIndexSettings returns the IndexSettings for the Korean shard.
//
// The Korean shard uses the same schema as the default index but is optimised
// for Lindera/Charabia Korean tokenisation in Meilisearch. Pre-tokenised text
// from the mecab-ko sidecar is written to the "tokens" field and included in
// searchable attributes for higher-precision Korean retrieval.
func KoreanIndexSettings() IndexSettings {
	return IndexSettings{
		SearchableAttributes: []string{
			"title",
			"body",
			"snippet",
			"author",
			"tokens", // pre-tokenised morphemes from mecab-ko sidecar
		},
		FilterableAttributes: []string{
			"lang",
			"doc_type",
			"source_id",
			"team_id",
			"published_at",
		},
		DistinctAttribute: "hash",
	}
}
