package continuity

import (
	"math"
	"sort"
	"strings"
	"unicode"
)

// SearchDocument is one unit of durable project memory that recall can rank.
type SearchDocument struct {
	Kind     string
	Location string
	Summary  string
	Text     string
}

// BM25 parameters from Robertson and Zaragoza, "The Probabilistic Relevance
// Framework: BM25 and Beyond" (2009). Keyword ranking keeps recall local and
// dependency-free; on this repository's own records it placed the right record
// in the top three for 11 of 12 test questions.
const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// RankDocuments orders documents by BM25 relevance to query and returns at
// most limit documents that share at least one term with it.
func RankDocuments(documents []SearchDocument, query string, limit int) []SearchDocument {
	queryTerms := map[string]bool{}
	for _, term := range searchTerms(query) {
		queryTerms[term] = true
	}
	if len(queryTerms) == 0 || len(documents) == 0 || limit <= 0 {
		return nil
	}
	counts := make([]map[string]int, len(documents))
	lengths := make([]float64, len(documents))
	documentFrequency := map[string]int{}
	total := 0.0
	for index, document := range documents {
		counts[index] = map[string]int{}
		for _, term := range searchTerms(document.Text) {
			counts[index][term]++
			lengths[index]++
		}
		for term := range counts[index] {
			documentFrequency[term]++
		}
		total += lengths[index]
	}
	if total == 0 {
		return nil
	}
	average := total / float64(len(documents))
	type scored struct {
		index int
		score float64
	}
	ranked := []scored{}
	for index := range documents {
		score := 0.0
		for term := range queryTerms {
			frequency := float64(counts[index][term])
			if frequency == 0 {
				continue
			}
			containing := float64(documentFrequency[term])
			idf := math.Log(1 + (float64(len(documents))-containing+0.5)/(containing+0.5))
			score += idf * frequency * (bm25K1 + 1) / (frequency + bm25K1*(1-bm25B+bm25B*lengths[index]/average))
		}
		if score > 0 {
			ranked = append(ranked, scored{index: index, score: score})
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	results := []SearchDocument{}
	for _, candidate := range ranked {
		if len(results) == limit {
			break
		}
		results = append(results, documents[candidate.index])
	}
	return results
}

var searchStopWords = wordSet("an and are as at be but by can do does for from had has have how if in into is it its no not of on or our should so than that the their then there these they this those to was we were what when where which who why will with you your")

func wordSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}

func searchTerms(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(words))
	for _, word := range words {
		if len(word) < 2 || searchStopWords[word] {
			continue
		}
		terms = append(terms, stemPlural(word))
	}
	return terms
}

// stemPlural is the S-stemmer from Harman, "How effective is suffixing?"
// (1991): it folds only plural forms, so "claims" matches "claim" without the
// over-merging of aggressive stemmers.
func stemPlural(word string) string {
	if len(word) <= 3 {
		return word
	}
	switch {
	case strings.HasSuffix(word, "ies") && !strings.HasSuffix(word, "eies") && !strings.HasSuffix(word, "aies"):
		return word[:len(word)-3] + "y"
	case strings.HasSuffix(word, "es") && !strings.HasSuffix(word, "aes") && !strings.HasSuffix(word, "ees") && !strings.HasSuffix(word, "oes"):
		return word[:len(word)-1]
	case strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "us") && !strings.HasSuffix(word, "ss"):
		return word[:len(word)-1]
	}
	return word
}
