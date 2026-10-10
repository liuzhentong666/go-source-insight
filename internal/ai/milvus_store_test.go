package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/milvus-io/milvus-proto/go-api/v2/commonpb"
	"github.com/milvus-io/milvus-proto/go-api/v2/milvuspb"
	"github.com/milvus-io/milvus-proto/go-api/v2/schemapb"
	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"google.golang.org/grpc"
)

func TestNewMilvusStore_InvalidArgs(t *testing.T) {
	tests := []struct {
		name       string
		address    string
		collection string
		vectorDim  int
		want       string
	}{
		{name: "empty address", address: "", collection: "code_segments", vectorDim: 1024, want: "address 为空"},
		{name: "blank address", address: "  ", collection: "code_segments", vectorDim: 1024, want: "address 为空"},
		{name: "empty collection", address: "localhost:19530", collection: "", vectorDim: 1024, want: "collection 为空"},
		{name: "blank collection", address: "localhost:19530", collection: "  ", vectorDim: 1024, want: "collection 为空"},
		{name: "zero dim", address: "localhost:19530", collection: "code_segments", vectorDim: 0, want: "vectorDim 必须 > 0"},
		{name: "negative dim", address: "localhost:19530", collection: "code_segments", vectorDim: -1, want: "vectorDim 必须 > 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewMilvusStore(context.Background(), tt.address, tt.collection, tt.vectorDim)
			if err == nil {
				if store != nil {
					_ = store.Close()
				}
				t.Fatal("expected error, got nil")
			}
			if store != nil {
				t.Fatalf("store = %#v, want nil", store)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want contains %q", err.Error(), tt.want)
			}
		})
	}
}

func TestMilvusStore_Insert_Validation(t *testing.T) {
	store := &MilvusStore{collection: "code_segments", vectorDim: 3}

	if err := store.Insert(context.Background(), nil, nil); err != nil {
		t.Fatalf("empty insert returned error: %v", err)
	}

	chunk := CodeChunk{ID: "main.go:1-2#0", Source: "main.go", Content: "package main", StartLine: 1, EndLine: 2, ChunkType: "file"}
	if err := store.Insert(context.Background(), []CodeChunk{chunk}, nil); err == nil || !strings.Contains(err.Error(), "数量不匹配") {
		t.Fatalf("length mismatch error = %v, want 数量不匹配", err)
	}
	if err := store.Insert(context.Background(), []CodeChunk{chunk}, [][]float32{{1, 2}}); err == nil || !strings.Contains(err.Error(), "向量维度 2 != 3") {
		t.Fatalf("dim mismatch error = %v, want 向量维度 2 != 3", err)
	}
	if err := store.Insert(context.Background(), []CodeChunk{chunk}, [][]float32{{}}); err == nil || !strings.Contains(err.Error(), "向量维度 0 != 3") {
		t.Fatalf("empty vector error = %v, want 向量维度 0 != 3", err)
	}
}

func TestMilvusStore_Search_Validation(t *testing.T) {
	store := &MilvusStore{collection: "code_segments", vectorDim: 3}

	if _, err := store.Search(context.Background(), nil, SearchOptions{}); err == nil || !strings.Contains(err.Error(), "查询向量为空") {
		t.Fatalf("nil query error = %v, want 查询向量为空", err)
	}
	if _, err := store.Search(context.Background(), []float32{}, SearchOptions{}); err == nil || !strings.Contains(err.Error(), "查询向量为空") {
		t.Fatalf("empty query error = %v, want 查询向量为空", err)
	}
	if _, err := store.Search(context.Background(), []float32{1, 2}, SearchOptions{}); err == nil || !strings.Contains(err.Error(), "查询向量维度 2 != 3") {
		t.Fatalf("dim mismatch error = %v, want 查询向量维度 2 != 3", err)
	}
}

func TestBuildFileFilterExpr(t *testing.T) {
	tests := []struct {
		name   string
		filter string
		want   string
	}{
		{name: "empty", filter: "", want: ""},
		{name: "blank", filter: "  ", want: ""},
		{name: "normal", filter: "internal/ai/scanner.go", want: `source == "internal/ai/scanner.go"`},
		{name: "trim", filter: "  internal/ai/scanner.go  ", want: `source == "internal/ai/scanner.go"`},
		{name: "quote", filter: `internal/ai/"bad".go`, want: `source == "internal/ai/\"bad\".go"`},
		{name: "backslash", filter: `C:\repo\main.go`, want: `source == "C:\\repo\\main.go"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildFileFilterExpr(tt.filter); got != tt.want {
				t.Fatalf("buildFileFilterExpr(%q) = %q, want %q", tt.filter, got, tt.want)
			}
		})
	}
}

func TestFindMissingFields(t *testing.T) {
	schema := &entity.Schema{Fields: []*entity.Field{
		entity.NewField().WithName(fieldChunkID),
		entity.NewField().WithName(fieldSource),
	}}

	missing := findMissingFields(schema, []string{fieldChunkID, fieldSource, fieldContent, fieldStartLine})
	want := []string{fieldContent, fieldStartLine}
	if len(missing) != len(want) {
		t.Fatalf("missing = %v, want %v", missing, want)
	}
	for i := range want {
		if missing[i] != want[i] {
			t.Fatalf("missing = %v, want %v", missing, want)
		}
	}

	allMissing := findMissingFields(nil, []string{fieldChunkID, fieldSource})
	if len(allMissing) != 2 || allMissing[0] != fieldChunkID || allMissing[1] != fieldSource {
		t.Fatalf("nil schema missing = %v, want all required fields", allMissing)
	}
}

func TestMapMilvusResults(t *testing.T) {
	raw := []client.SearchResult{
		{
			ResultCount: 2,
			Fields: client.ResultSet{
				entity.NewColumnVarChar(fieldChunkID, []string{"main.go:10-20#0", "main.go:30-40#1"}),
				entity.NewColumnVarChar(fieldSource, []string{"main.go", "main.go"}),
				entity.NewColumnVarChar(fieldContent, []string{"func A() {}", "func B() {}"}),
				entity.NewColumnInt32(fieldStartLine, []int32{10, 30}),
				entity.NewColumnInt32(fieldEndLine, []int32{20, 40}),
				entity.NewColumnVarChar(fieldChunkType, []string{"function", "function"}),
			},
			Scores: []float32{0.91, 0.82},
		},
	}

	results, err := mapMilvusResults(raw)
	if err != nil {
		t.Fatalf("mapMilvusResults returned error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].ID != "main.go:10-20#0" || results[0].Source != "main.go" || results[0].Content != "func A() {}" {
		t.Fatalf("results[0] basic fields = %#v", results[0])
	}
	if results[0].StartLine != 10 || results[0].EndLine != 20 || results[0].ChunkType != "function" || results[0].Score != 0.91 {
		t.Fatalf("results[0] reference fields = %#v", results[0])
	}
	if results[1].ID != "main.go:30-40#1" || results[1].StartLine != 30 || results[1].EndLine != 40 || results[1].Score != 0.82 {
		t.Fatalf("results[1] = %#v", results[1])
	}
}

func TestMapMilvusResults_MissingFieldsUseZeroValues(t *testing.T) {
	raw := []client.SearchResult{
		{
			ResultCount: 1,
			Fields: client.ResultSet{
				entity.NewColumnVarChar(fieldSource, []string{"main.go"}),
			},
			Scores: nil,
		},
	}

	results, err := mapMilvusResults(raw)
	if err != nil {
		t.Fatalf("mapMilvusResults returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Source != "main.go" {
		t.Fatalf("Source = %q, want main.go", results[0].Source)
	}
	if results[0].ID != "" || results[0].Content != "" || results[0].StartLine != 0 || results[0].EndLine != 0 || results[0].ChunkType != "" || results[0].Score != 0 {
		t.Fatalf("missing fields result = %#v, want zero values", results[0])
	}
}

func TestMapMilvusResults_PropagatesResultErr(t *testing.T) {
	wantErr := fmt.Errorf("parse field failed: schema mismatch")
	raw := []client.SearchResult{
		{
			ResultCount: 2,
			Err:         wantErr,
		},
	}

	results, err := mapMilvusResults(raw)
	if err == nil {
		t.Fatalf("expected error wrapping r.Err, got nil; results=%v", results)
	}
	if !strings.Contains(err.Error(), "解析失败") {
		t.Fatalf("error = %q, want contains 解析失败", err.Error())
	}
	if !strings.Contains(err.Error(), "parse field failed: schema mismatch") {
		t.Fatalf("error = %q, want contains wrapped r.Err message", err.Error())
	}
	if results != nil {
		t.Fatalf("results = %v, want nil on error", results)
	}
}

// Embedding client.Client lets a fake implement only the operations under test.
// An unexpected SDK operation panics instead of accidentally using a live server.
type fakeMilvusClient struct {
	client.Client
	has               bool
	schema            *entity.Schema
	events            []string
	rows              int
	deleteErr         error
	insertErr         error
	flushErr          error
	searchConsistency entity.ConsistencyLevel
	rawSearch         []client.SearchResult
	queryResult       client.ResultSet
	queryErr          error
	queryCalls        int
	queryExpression   string
	queryConsistency  entity.ConsistencyLevel
}

func (f *fakeMilvusClient) HasCollection(_ context.Context, name string) (bool, error) {
	f.events = append(f.events, "has")
	if strings.HasPrefix(name, "gsi_active_") {
		return false, nil
	}
	return f.has, nil
}

func (f *fakeMilvusClient) DescribeCollection(context.Context, string) (*entity.Collection, error) {
	f.events = append(f.events, "describe")
	return &entity.Collection{Schema: f.schema}, nil
}

func (f *fakeMilvusClient) CreateCollection(_ context.Context, schema *entity.Schema, _ int32, _ ...client.CreateCollectionOption) error {
	f.events = append(f.events, "create")
	f.has, f.schema = true, schema
	return nil
}

func (f *fakeMilvusClient) CreateIndex(context.Context, string, string, entity.Index, bool, ...client.IndexOption) error {
	f.events = append(f.events, "index")
	return nil
}

func (f *fakeMilvusClient) DescribeIndex(context.Context, string, string, ...client.IndexOption) ([]entity.Index, error) {
	f.events = append(f.events, "describe-index")
	index, _ := entity.NewIndexHNSW(entity.COSINE, hnswM, hnswEfConstruction)
	return []entity.Index{index}, nil
}

func (f *fakeMilvusClient) LoadCollection(context.Context, string, bool, ...client.LoadCollectionOption) error {
	f.events = append(f.events, "load")
	return nil
}

func (f *fakeMilvusClient) Delete(_ context.Context, _ string, _ string, expression string) error {
	f.events = append(f.events, "delete")
	if expression != fieldPK+" >= 0" {
		return fmt.Errorf("unexpected delete expression %q", expression)
	}
	if f.deleteErr == nil {
		f.rows = 0
	}
	return f.deleteErr
}

func (f *fakeMilvusClient) Insert(_ context.Context, _ string, _ string, columns ...entity.Column) (entity.Column, error) {
	f.events = append(f.events, "insert")
	if f.insertErr == nil {
		f.rows += columns[0].Len()
	}
	return nil, f.insertErr
}

func (f *fakeMilvusClient) Flush(context.Context, string, bool, ...client.FlushOption) error {
	f.events = append(f.events, "flush")
	return f.flushErr
}

func (f *fakeMilvusClient) Search(_ context.Context, _ string, _ []string, _ string, _ []string, _ []entity.Vector, _ string, _ entity.MetricType, _ int, _ entity.SearchParam, options ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {
	f.events = append(f.events, "search")
	var opts client.SearchQueryOption
	for _, option := range options {
		option(&opts)
	}
	f.searchConsistency = opts.ConsistencyLevel
	return f.rawSearch, nil
}

func (f *fakeMilvusClient) Query(_ context.Context, _ string, _ []string, expression string, _ []string, options ...client.SearchQueryOptionFunc) (client.ResultSet, error) {
	f.queryCalls++
	f.queryExpression = expression
	var opts client.SearchQueryOption
	for _, option := range options {
		option(&opts)
	}
	f.queryConsistency = opts.ConsistencyLevel
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	if f.queryResult != nil {
		return f.queryResult, nil
	}
	return client.ResultSet{entity.NewColumnInt64("count(*)", []int64{int64(f.rows)})}, nil
}

func newTestMilvusStore() (*MilvusStore, *fakeMilvusClient) {
	fake := &fakeMilvusClient{has: true, schema: codeCollectionSchema("code_segments", 2), rows: 4}
	return &MilvusStore{client: fake, collection: "code_segments", vectorDim: 2}, fake
}

func TestMilvusStore_ReplaceInvalidInputNeverDeletesExistingData(t *testing.T) {
	valid := CodeChunk{ID: "main.go:1-1#0", Source: "main.go", Content: "package demo", StartLine: 1, EndLine: 1, ChunkType: "file"}
	for _, problem := range []string{"dimension", "content length", "missing source", "bad lines"} {
		t.Run(problem, func(t *testing.T) {
			store, fake := newTestMilvusStore()
			chunk, vector := valid, []float32{1, 0}
			switch problem {
			case "dimension":
				vector = []float32{1}
			case "content length":
				chunk.Content = strings.Repeat("x", maxLenContent+1)
			case "missing source":
				chunk.Source = ""
			case "bad lines":
				chunk.StartLine, chunk.EndLine = 5, 1
			}
			if err := store.Replace(context.Background(), []CodeChunk{chunk}, [][]float32{vector}); err == nil {
				t.Fatal("expected validation error")
			}
			if fake.rows != 4 || len(fake.events) != 0 {
				t.Fatalf("invalid input touched stored data: rows=%d operations=%v", fake.rows, fake.events)
			}
		})
	}
}

func TestMilvusStore_SearchLoadsExistingCollectionWithoutReindexing(t *testing.T) {
	store, fake := newTestMilvusStore()
	if _, err := store.Search(context.Background(), []float32{1, 0}, SearchOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(fake.events, ","); got != "has,describe,describe-index,load,search" {
		t.Fatalf("search operation ordering = %q", got)
	}
	if fake.searchConsistency != entity.ClStrong {
		t.Fatal("ask must see the latest completed index across separate CLI processes")
	}
	store, fake = newTestMilvusStore()
	fake.has = false
	if _, err := store.Search(context.Background(), []float32{1, 0}, SearchOptions{}); err == nil || !strings.Contains(err.Error(), "请先运行 index") {
		t.Fatalf("missing index error = %v", err)
	}
	if strings.Join(fake.events, ",") != "has" {
		t.Fatalf("ask created a missing collection: %v", fake.events)
	}
}

func TestMilvusStore_ExistingSchemaValidation(t *testing.T) {
	for _, problem := range []string{"missing field", "wrong type", "wrong dimension", "wrong primary key", "content too short", "extra required field"} {
		t.Run(problem, func(t *testing.T) {
			store, fake := newTestMilvusStore()
			switch problem {
			case "missing field":
				fake.schema.Fields = fake.schema.Fields[1:]
			case "wrong type":
				fake.schema.Fields[2].DataType = entity.FieldTypeInt64
			case "wrong dimension":
				fake.schema.Fields[len(fake.schema.Fields)-1].TypeParams["dim"] = "1024"
			case "wrong primary key":
				fake.schema.Fields[0].AutoID = false
			case "content too short":
				fake.schema.Fields[3].TypeParams["max_length"] = "10000"
			case "extra required field":
				fake.schema.Fields = append(fake.schema.Fields, entity.NewField().WithName("extra").WithDataType(entity.FieldTypeInt64))
			}
			if err := store.EnsureCollection(context.Background()); err == nil || !strings.Contains(err.Error(), "schema 不兼容") {
				t.Fatalf("schema error = %v", err)
			}
			if fake.rows != 4 || strings.Join(fake.events, ",") != "has,describe" {
				t.Fatalf("schema mismatch altered data: rows=%d operations=%v", fake.rows, fake.events)
			}
		})
	}
}

func TestMilvusStore_ReplaceRejectsExtraRequiredFieldsBeforeDeleting(t *testing.T) {
	store, fake := newTestMilvusStore()
	fake.schema.Fields = append(fake.schema.Fields, entity.NewField().WithName("extra").WithDataType(entity.FieldTypeInt64))
	chunk := CodeChunk{ID: "main.go:1-1#0", Source: "main.go", Content: "package demo", StartLine: 1, EndLine: 1, ChunkType: "file"}
	if err := store.Replace(context.Background(), []CodeChunk{chunk}, [][]float32{{1, 0}}); err == nil || !strings.Contains(err.Error(), "额外必填字段") {
		t.Fatalf("extra required field error = %v", err)
	}
	if fake.rows != 4 || strings.Join(fake.events, ",") != "has,has,describe" {
		t.Fatalf("existing index was touched: rows=%d operations=%v", fake.rows, fake.events)
	}
}

func TestEmptySearchCompatibilityCandidateIsStrict(t *testing.T) {
	exactError := fmt.Errorf("extra output fields [%s] found and result does not dynamic field", strings.Join(codeSearchOutputFields(), " "))
	for _, test := range []struct {
		name      string
		result    client.SearchResult
		wantError bool
	}{
		{name: "nil IDs and all fixed fields absent", result: client.SearchResult{Err: exactError}},
		{name: "empty IDs and all fixed fields absent", result: client.SearchResult{IDs: entity.NewColumnInt64(fieldPK, nil), Err: exactError}},
		{name: "different field order", result: client.SearchResult{Err: errors.New("extra output fields [source content start_line end_line chunk_type chunk_id] found and result does not dynamic field")}},
		{name: "zero scalar payload", result: client.SearchResult{Err: errors.New("FieldData type not matched")}},
		{name: "arbitrary zero-row parser error", result: client.SearchResult{Err: errors.New("parse field failed: schema mismatch")}, wantError: true},
		{name: "nonempty result", result: client.SearchResult{ResultCount: 1, Err: exactError}, wantError: true},
		{name: "nonempty IDs", result: client.SearchResult{IDs: entity.NewColumnInt64(fieldPK, []int64{1}), Err: exactError}, wantError: true},
		{name: "nonempty scores", result: client.SearchResult{Scores: []float32{0.9}, Err: exactError}, wantError: true},
		{name: "unexpected column", result: client.SearchResult{Fields: client.ResultSet{entity.NewColumnVarChar(fieldSource, nil)}, Err: exactError}, wantError: true},
		{name: "unexpected group by", result: client.SearchResult{GroupByValue: entity.NewColumnInt64("group", nil), Err: exactError}, wantError: true},
		{name: "missing fixed field", result: client.SearchResult{Err: errors.New("extra output fields [source content start_line end_line chunk_type] found and result does not dynamic field")}, wantError: true},
		{name: "unknown fixed field", result: client.SearchResult{Err: errors.New("extra output fields [source content start_line end_line chunk_type unknown] found and result does not dynamic field")}, wantError: true},
		{name: "duplicate fixed field", result: client.SearchResult{Err: errors.New("extra output fields [source content start_line end_line chunk_type source] found and result does not dynamic field")}, wantError: true},
		{name: "prefixed parser error", result: client.SearchResult{Err: fmt.Errorf("another failure: %w", exactError)}, wantError: true},
		{name: "suffixed parser error", result: client.SearchResult{Err: fmt.Errorf("%s: another failure", exactError)}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if isEmptySearchCompatibilityError(test.result) == test.wantError {
				t.Fatalf("compatibility candidate mismatch: %+v", test.result)
			}
			results, err := mapMilvusResults([]client.SearchResult{test.result})
			if err == nil {
				t.Fatal("mapping must never suppress unverified parser errors")
			}
			if len(results) != 0 {
				t.Fatalf("expected no mapped rows, got %v", results)
			}
		})
	}
}

// Feed the exact Milvus 2.5 zero-row protobuf shape through the real v2.4 SDK
// parser. This catches the regression without mocking the parser's error value
// or requiring a network connection, collection mutation, or a model service.
type searchResponseRPC struct {
	milvuspb.MilvusServiceClient
	response  *milvuspb.SearchResults
	count     int64
	countErr  error
	lastQuery *milvuspb.QueryRequest
}

func (rpc *searchResponseRPC) DescribeCollection(_ context.Context, request *milvuspb.DescribeCollectionRequest, _ ...grpc.CallOption) (*milvuspb.DescribeCollectionResponse, error) {
	return &milvuspb.DescribeCollectionResponse{
		Status:           &commonpb.Status{ErrorCode: commonpb.ErrorCode_Success},
		Schema:           codeCollectionSchema(request.CollectionName, 2).ProtoMessage(),
		ConsistencyLevel: commonpb.ConsistencyLevel_Strong,
	}, nil
}

func (rpc *searchResponseRPC) Search(_ context.Context, _ *milvuspb.SearchRequest, _ ...grpc.CallOption) (*milvuspb.SearchResults, error) {
	return rpc.response, nil
}

func (rpc *searchResponseRPC) Query(_ context.Context, request *milvuspb.QueryRequest, _ ...grpc.CallOption) (*milvuspb.QueryResults, error) {
	rpc.lastQuery = request
	if rpc.countErr != nil {
		return nil, rpc.countErr
	}
	return &milvuspb.QueryResults{
		Status:     &commonpb.Status{ErrorCode: commonpb.ErrorCode_Success},
		FieldsData: []*schemapb.FieldData{entity.NewColumnInt64("count(*)", []int64{rpc.count}).FieldData()},
	}, nil
}

func TestMilvusStore_Milvus25ZeroRowResponseThroughRealSDK(t *testing.T) {
	rpc := &searchResponseRPC{response: &milvuspb.SearchResults{
		Status: &commonpb.Status{ErrorCode: commonpb.ErrorCode_Success},
		Results: &schemapb.SearchResultData{
			NumQueries: 1,
			Topks:      []int64{0},
			Ids:        &schemapb.IDs{IdField: &schemapb.IDs_IntId{IntId: &schemapb.LongArray{}}},
			// Scores and FieldsData are omitted for a legitimately empty search.
		},
	}}
	sdk := &client.GrpcClient{Service: rpc}
	collection := "wire_empty_search_compatibility"
	params, _ := entity.NewIndexHNSWSearchParam(hnswSearchEf)
	raw, err := sdk.Search(context.Background(), collection, nil, `source == "deleted.go"`, codeSearchOutputFields(), []entity.Vector{entity.FloatVector{1, 0}}, fieldVector, entity.COSINE, 3, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 1 || raw[0].ResultCount != 0 || raw[0].Err == nil || len(raw[0].Fields) != 0 || len(raw[0].Scores) != 0 || raw[0].IDs.Len() != 0 {
		t.Fatalf("did not reproduce the SDK zero-row parser bug: %+v", raw)
	}
	store := &MilvusStore{client: sdk, collection: collection, vectorDim: 2, ready: true}
	model := &fakeModel{content: "must not be called"}
	service := newTestRAGService(nil, nil, &fakeEmbedder{queryVector: []float32{1, 0}}, store, model)
	answer, err := service.Ask(context.Background(), "what was deleted?", "deleted.go")
	if err != nil {
		t.Fatal(err)
	}
	if answer.Answer != "" || len(answer.Sources) != 0 || len(model.lastMessages) != 0 {
		t.Fatalf("empty wire result must not invoke the chat model: %+v", answer)
	}
	if rpc.lastQuery == nil || rpc.lastQuery.Expr != `source == "deleted.go"` || rpc.lastQuery.GuaranteeTimestamp != client.StrongTimestamp {
		t.Fatalf("zero-row confirmation did not preserve the filter and Strong consistency: %+v", rpc.lastQuery)
	}
	// An empty snapshot can instead return named/typed scalar descriptors with
	// no scalar oneof payload. The real SDK then reports a different parser error.
	rpc.response.Results.Ids = &schemapb.IDs{}
	rpc.response.Results.FieldsData = []*schemapb.FieldData{
		{FieldName: fieldChunkID, Type: schemapb.DataType_VarChar, Field: &schemapb.FieldData_Scalars{Scalars: &schemapb.ScalarField{}}},
		{FieldName: fieldSource, Type: schemapb.DataType_VarChar},
		{FieldName: fieldContent, Type: schemapb.DataType_VarChar},
		{FieldName: fieldStartLine, Type: schemapb.DataType_Int32},
		{FieldName: fieldEndLine, Type: schemapb.DataType_Int32},
		{FieldName: fieldChunkType, Type: schemapb.DataType_VarChar},
	}
	raw, err = sdk.Search(context.Background(), collection, nil, "", codeSearchOutputFields(), []entity.Vector{entity.FloatVector{1, 0}}, fieldVector, entity.COSINE, 3, params)
	if err != nil || len(raw) != 1 || raw[0].Err == nil || raw[0].Err.Error() != "FieldData type not matched" {
		t.Fatalf("did not reproduce the omitted scalar oneof parser bug: raw=%+v err=%v", raw, err)
	}
	answer, err = service.Ask(context.Background(), "what remains?", "")
	if err != nil || answer.Answer != "" || len(answer.Sources) != 0 || len(model.lastMessages) != 0 {
		t.Fatalf("empty snapshot response: answer=%+v err=%v", answer, err)
	}
	if rpc.lastQuery.Expr != fieldPK+" >= 0" || rpc.lastQuery.GuaranteeTimestamp != client.StrongTimestamp {
		t.Fatalf("empty snapshot confirmation filter/consistency incorrect: %+v", rpc.lastQuery)
	}
	rpc.count = 1
	if _, err := service.Ask(context.Background(), "what remains?", ""); err == nil || !strings.Contains(err.Error(), "FieldData type not matched") {
		t.Fatalf("a nonzero confirmation must preserve the parser error: %v", err)
	}
	rpc.count = 0
	rpc.countErr = errors.New("confirmation unavailable")
	if _, err := service.Ask(context.Background(), "what remains?", ""); err == nil || !strings.Contains(err.Error(), "FieldData type not matched") || !errors.Is(err, rpc.countErr) {
		t.Fatalf("confirmation failure must preserve both errors: %v", err)
	}
	rpc.countErr = nil
	// The same omitted fields are a real schema/response problem for a nonempty
	// response and must still propagate the SDK parser error.
	rpc.response.Results.Topks = []int64{1}
	rpc.response.Results.Scores = []float32{0.9}
	rpc.response.Results.Ids = &schemapb.IDs{IdField: &schemapb.IDs_IntId{IntId: &schemapb.LongArray{Data: []int64{1}}}}
	if _, err := service.Ask(context.Background(), "what was deleted?", "deleted.go"); err == nil || !strings.Contains(err.Error(), "解析失败") {
		t.Fatalf("nonempty malformed wire result must remain an error: %v", err)
	}
}

func TestMilvusStore_EmptyParserErrorRequiresValidZeroCount(t *testing.T) {
	parserErr := errors.New("FieldData type not matched")
	for _, test := range []struct {
		name      string
		rows      int
		result    client.ResultSet
		queryErr  error
		searchErr error
		filter    string
		wantError bool
		wantQuery bool
	}{
		{name: "confirmed empty", wantQuery: true},
		{name: "filtered empty", filter: "deleted.go", wantQuery: true},
		{name: "count nonzero", rows: 1, wantError: true, wantQuery: true},
		{name: "query failure", queryErr: errors.New("unavailable"), wantError: true, wantQuery: true},
		{name: "invalid count type", result: client.ResultSet{entity.NewColumnInt32("count(*)", []int32{0})}, wantError: true, wantQuery: true},
		{name: "invalid count name", result: client.ResultSet{entity.NewColumnInt64("count", []int64{0})}, wantError: true, wantQuery: true},
		{name: "missing count value", result: client.ResultSet{entity.NewColumnInt64("count(*)", nil)}, wantError: true, wantQuery: true},
		{name: "negative count", result: client.ResultSet{entity.NewColumnInt64("count(*)", []int64{-1})}, wantError: true, wantQuery: true},
		{name: "arbitrary parser error", searchErr: errors.New("schema broken"), wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, fake := newTestMilvusStore()
			fake.rows, fake.queryResult, fake.queryErr = test.rows, test.result, test.queryErr
			original := parserErr
			if test.searchErr != nil {
				original = test.searchErr
			}
			fake.rawSearch = []client.SearchResult{{Err: original}}
			results, err := store.Search(context.Background(), []float32{1, 0}, SearchOptions{FileFilter: test.filter})
			if (err != nil) != test.wantError || len(results) != 0 {
				t.Fatalf("results=%v err=%v", results, err)
			}
			if test.wantError && !errors.Is(err, original) {
				t.Fatalf("original parser error was lost: %v", err)
			}
			if (fake.queryCalls != 0) != test.wantQuery {
				t.Fatalf("unexpected count confirmation calls: %d", fake.queryCalls)
			}
			if test.wantQuery {
				wantExpression := fieldPK + " >= 0"
				if test.filter != "" {
					wantExpression = buildFileFilterExpr(test.filter)
				}
				if fake.queryExpression != wantExpression || fake.queryConsistency != entity.ClStrong {
					t.Fatalf("count filter/consistency incorrect: %q / %v", fake.queryExpression, fake.queryConsistency)
				}
			}
		})
	}
}
