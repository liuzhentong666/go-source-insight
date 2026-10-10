package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// Tracks each physical generation separately, including failures after an RPC
// committed. This deliberately does not share the implementation's state.
type snapshotTestClient struct {
	*fakeMilvusClient
	schemas   map[string]*entity.Schema
	counts    map[string]int
	aliases   map[string]string
	fail      string
	batches   []int
	inserts   int
	committed bool
}

func newSnapshotTestStore() (*MilvusStore, *snapshotTestClient) {
	f := &snapshotTestClient{fakeMilvusClient: &fakeMilvusClient{}, schemas: map[string]*entity.Schema{}, counts: map[string]int{}, aliases: map[string]string{}}
	f.schemas["legacy"] = codeCollectionSchema("legacy", 2)
	f.counts["legacy"] = 4
	return &MilvusStore{client: f, collection: "legacy", logical: "legacy", vectorDim: 2}, f
}
func (f *snapshotTestClient) physical(n string) string {
	if p := f.aliases[n]; p != "" {
		return p
	}
	return n
}
func (f *snapshotTestClient) failure(stage string) error {
	if f.fail == stage {
		return fmt.Errorf("injected %s failure", stage)
	}
	return nil
}
func (f *snapshotTestClient) HasCollection(_ context.Context, n string) (bool, error) {
	return f.schemas[f.physical(n)] != nil, nil
}
func (f *snapshotTestClient) DescribeCollection(_ context.Context, n string) (*entity.Collection, error) {
	s := f.schemas[f.physical(n)]
	if s == nil {
		return nil, errors.New("missing collection")
	}
	return &entity.Collection{Schema: s}, nil
}
func (f *snapshotTestClient) CreateCollection(_ context.Context, s *entity.Schema, _ int32, _ ...client.CreateCollectionOption) error {
	if e := f.failure("create"); e != nil {
		return e
	}
	f.schemas[s.CollectionName] = s
	f.counts[s.CollectionName] = 0
	return nil
}
func (f *snapshotTestClient) DropCollection(_ context.Context, n string, _ ...client.DropCollectionOption) error {
	delete(f.schemas, n)
	delete(f.counts, n)
	return nil
}
func (f *snapshotTestClient) Insert(_ context.Context, n, _ string, c ...entity.Column) (entity.Column, error) {
	f.inserts++
	if f.fail == "insert" || f.fail == "second batch" && f.inserts == 2 {
		return nil, errors.New("injected insert failure")
	}
	f.batches = append(f.batches, c[0].Len())
	f.counts[n] += c[0].Len()
	return nil, nil
}
func (f *snapshotTestClient) Flush(context.Context, string, bool, ...client.FlushOption) error {
	return f.failure("flush")
}
func (f *snapshotTestClient) CreateIndex(context.Context, string, string, entity.Index, bool, ...client.IndexOption) error {
	return f.failure("index")
}
func (f *snapshotTestClient) LoadCollection(context.Context, string, bool, ...client.LoadCollectionOption) error {
	return f.failure("load")
}
func (f *snapshotTestClient) ReleaseCollection(context.Context, string, ...client.ReleaseCollectionOption) error {
	return nil
}
func (f *snapshotTestClient) Query(_ context.Context, n string, _ []string, _ string, _ []string, _ ...client.SearchQueryOptionFunc) (client.ResultSet, error) {
	if e := f.failure("count"); e != nil {
		return nil, e
	}
	count := f.counts[f.physical(n)]
	if f.fail == "wrong count" {
		count++
	}
	return client.ResultSet{entity.NewColumnInt64("count(*)", []int64{int64(count)})}, nil
}
func (f *snapshotTestClient) CreateAlias(_ context.Context, n, a string) error {
	if f.committed {
		f.aliases[a] = n
		return context.DeadlineExceeded
	}
	if e := f.failure("publish"); e != nil {
		return e
	}
	f.aliases[a] = n
	return nil
}
func (f *snapshotTestClient) AlterAlias(ctx context.Context, n, a string) error {
	return f.CreateAlias(ctx, n, a)
}
func snapshotChunks(n int, bytes int) ([]CodeChunk, [][]float32) {
	c := make([]CodeChunk, n)
	v := make([][]float32, n)
	for i := range c {
		c[i] = CodeChunk{ID: fmt.Sprint(i), Source: "sample.go", Content: strings.Repeat("x", bytes), StartLine: 1, EndLine: 1, ChunkType: "file"}
		v[i] = []float32{1, 0}
	}
	return c, v
}

func TestSnapshotFailuresPreservePreviousGeneration(t *testing.T) {
	for _, stage := range []string{"create", "insert", "second batch", "flush", "index", "load", "count", "wrong count", "publish"} {
		t.Run(stage, func(t *testing.T) {
			s, f := newSnapshotTestStore()
			c, v := snapshotChunks(1, 20)
			if e := s.Replace(context.Background(), c, v); e != nil {
				t.Fatal(e)
			}
			old := f.aliases[snapshotAlias("legacy")]
			f.fail = stage
			f.inserts = 0
			c, v = snapshotChunks(160, 60000)
			if e := s.Replace(context.Background(), c, v); e == nil {
				t.Fatal("expected failure")
			}
			if f.aliases[snapshotAlias("legacy")] != old || f.counts[old] != 1 || f.counts["legacy"] != 4 {
				t.Fatal("previous data changed")
			}
			if stage != "publish" && len(f.schemas) != 2 {
				t.Fatalf("unpublished snapshot leaked: %d", len(f.schemas))
			}
		})
	}
}
func TestSnapshotBatchesMigrationAndFreshReader(t *testing.T) {
	s, f := newSnapshotTestStore()
	c, v := snapshotChunks(1200, 60000)
	if e := s.Replace(context.Background(), c, v); e != nil {
		t.Fatal(e)
	}
	if len(f.batches) < 2 {
		t.Fatal("oversized data not batched")
	}
	total := 0
	for _, n := range f.batches {
		total += n
		if n*60000 > maxInsertBatchBytes {
			t.Fatal("batch exceeds limit")
		}
	}
	if total != 1200 || f.counts["legacy"] != 4 {
		t.Fatal("incomplete replacement or legacy changed")
	}
	reader := &MilvusStore{client: f, collection: "legacy", logical: "legacy", vectorDim: 2}
	p, alias, e := reader.existingSnapshot(context.Background())
	if e != nil || !alias || f.counts[p] != 1200 {
		t.Fatalf("fresh reader failed: %s %v %v", p, alias, e)
	}
	if e = s.Replace(context.Background(), nil, nil); e != nil {
		t.Fatal(e)
	}
	p, _, e = reader.existingSnapshot(context.Background())
	if e != nil || f.counts[p] != 0 {
		t.Fatal("empty snapshot not published")
	}
}
func TestSnapshotPublicationTimeoutConfirmed(t *testing.T) {
	s, f := newSnapshotTestStore()
	f.committed = true
	c, v := snapshotChunks(1, 20)
	if e := s.Replace(context.Background(), c, v); e != nil {
		t.Fatal(e)
	}
	p := f.aliases[snapshotAlias("legacy")]
	if f.schemas[p] == nil || f.counts[p] != 1 {
		t.Fatal("committed snapshot lost on timeout")
	}
}
