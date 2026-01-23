package ai

import (
	"github.com/tmc/langchaingo/schema"
	"github.com/tmc/langchaingo/textsplitter"
)

func SplitDocs(docs []schema.Document) ([]schema.Document, error) {
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(500),
		textsplitter.WithChunkOverlap(50))
	chunks, err := textsplitter.SplitDocuments(splitter, docs)
	if err != nil {
		return nil, err
	}
	return chunks, nil
}
