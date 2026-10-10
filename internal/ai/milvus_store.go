package ai

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// 编译期接口断言：MilvusStore 必须实现 VectorStore。
var _ VectorStore = (*MilvusStore)(nil)

// Milvus schema 字段名，作为源码级常量集中管理，避免拼写错误散落各处。
const (
	fieldPK        = "id"
	fieldChunkID   = "chunk_id"
	fieldSource    = "source"
	fieldContent   = "content"
	fieldStartLine = "start_line"
	fieldEndLine   = "end_line"
	fieldChunkType = "chunk_type"
	fieldVector    = "vector"

	// Milvus VarChar 字段的 UTF-8 字节限制。
	maxLenChunkID   = 512
	maxLenSource    = 512
	maxLenContent   = 65535
	maxLenChunkType = 64

	// HNSW 索引默认参数（与既有 InitCode 保持一致，避免行为漂移）。
	hnswM              = 16
	hnswEfConstruction = 64
	hnswSearchEf       = 64

	// TopK 兜底值：Search 传入非法 TopK 时使用。
	defaultTopK = 3
)

// MilvusStore 是 Milvus 上 VectorStore 接口的薄封装。
// 不做连接池、重试、并发缓存：v0.1 单线程 CLI 场景，快速失败优于隐藏错误。
type MilvusStore struct {
	client     client.Client
	collection string
	// logical is set on public stores; internal stores address one immutable snapshot.
	logical   string
	vectorDim int
	ready     bool
}

var collectionNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,254}$`)

// NewMilvusStore 建立 Milvus 连接并返回 store 实例。
//
// 参数校验先于网络连接：address/collection 为空或 vectorDim<=0 直接返回参数错误，
// 让"配置错"和"网络错"在错误信息上可区分。
func NewMilvusStore(ctx context.Context, address, collection string, vectorDim int) (*MilvusStore, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("milvus: address 为空")
	}
	if strings.TrimSpace(collection) == "" {
		return nil, fmt.Errorf("milvus: collection 为空")
	}
	if !collectionNamePattern.MatchString(collection) {
		return nil, fmt.Errorf("milvus: collection 名称必须以字母或下划线开头，只包含字母、数字和下划线，长度不超过 255")
	}
	if vectorDim <= 0 {
		return nil, fmt.Errorf("milvus: vectorDim 必须 > 0，实际为 %d", vectorDim)
	}

	c, err := client.NewClient(ctx, client.Config{Address: address})
	if err != nil {
		return nil, fmt.Errorf("milvus: 连接 %s 失败: %w", address, err)
	}

	return &MilvusStore{
		client:     c,
		collection: collection,
		logical:    collection,
		vectorDim:  vectorDim,
	}, nil
}

// requiredFields 列出 collection 必须具备的字段。
var requiredFields = []string{
	fieldPK,
	fieldChunkID,
	fieldSource,
	fieldContent,
	fieldStartLine,
	fieldEndLine,
	fieldChunkType,
	fieldVector,
}

// EnsureCollection validates the current logical index without modifying it.
// Replace creates and prepares the next snapshot. Internal physical stores can
// still use this method to prepare an individual collection.
func (s *MilvusStore) EnsureCollection(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("milvus: client 未初始化")
	}
	if s.logical != "" {
		_, _, err := s.existingSnapshot(ctx)
		return err
	}
	return s.prepareCollection(ctx, true)
}

// prepareCollection loads and checks the existing index for a fresh ask process.
// Read-only callers never create a missing collection.
func (s *MilvusStore) prepareCollection(ctx context.Context, allowCreate bool) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("milvus: client 未初始化")
	}
	if s.ready {
		return nil
	}
	has, err := s.client.HasCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("milvus: 检查 collection %q 失败: %w", s.collection, err)
	}
	if !has {
		if !allowCreate {
			return fmt.Errorf("milvus: collection %q 尚未建立索引，请先运行 index", s.collection)
		}
		return s.createCollection(ctx)
	}

	coll, err := s.client.DescribeCollection(ctx, s.collection)
	if err != nil {
		return fmt.Errorf("milvus: DescribeCollection %q 失败: %w", s.collection, err)
	}
	if coll == nil {
		return fmt.Errorf("milvus: collection %q 返回空 schema", s.collection)
	}
	if err := validateCollectionSchema(coll.Schema, s.vectorDim); err != nil {
		return fmt.Errorf("milvus: collection %q schema 不兼容: %w；请选择新的 collection 并重新 index", s.collection, err)
	}
	if allowCreate {
		// CreateIndex is idempotent for an identical definition and repairs an interrupted setup.
		if err := s.createVectorIndex(ctx); err != nil {
			return err
		}
	} else {
		indexes, err := s.client.DescribeIndex(ctx, s.collection, fieldVector)
		if err != nil {
			return fmt.Errorf("milvus: collection %q 向量索引不可用，请先运行 index: %w", s.collection, err)
		}
		if len(indexes) == 0 {
			return fmt.Errorf("milvus: collection %q 缺少向量索引，请先运行 index", s.collection)
		}
		for _, index := range indexes {
			if index.IndexType() != entity.HNSW || index.Params()["metric_type"] != string(entity.COSINE) {
				return fmt.Errorf("milvus: collection %q 需要 HNSW/COSINE 索引，请使用新的 collection 重新 index", s.collection)
			}
		}
	}

	if err := s.client.LoadCollection(ctx, s.collection, false); err != nil {
		return fmt.Errorf("milvus: LoadCollection %q 失败: %w", s.collection, err)
	}
	s.ready = true
	return nil
}

// createCollection 建 collection + 建索引 + 加载到内存。
func (s *MilvusStore) createCollection(ctx context.Context) error {
	schema := codeCollectionSchema(s.collection, s.vectorDim)
	if err := s.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return fmt.Errorf("milvus: CreateCollection %q 失败: %w", s.collection, err)
	}
	if err := s.createVectorIndex(ctx); err != nil {
		return err
	}
	if err := s.client.LoadCollection(ctx, s.collection, false); err != nil {
		return fmt.Errorf("milvus: LoadCollection %q 失败: %w", s.collection, err)
	}
	s.ready = true
	return nil
}

func codeCollectionSchema(collection string, vectorDim int) *entity.Schema {
	return &entity.Schema{
		CollectionName: collection,
		Description:    "GoSource-Insight code chunks",
		Fields: []*entity.Field{
			entity.NewField().WithName(fieldPK).WithDataType(entity.FieldTypeInt64).WithIsPrimaryKey(true).WithIsAutoID(true),
			entity.NewField().WithName(fieldChunkID).WithDataType(entity.FieldTypeVarChar).WithMaxLength(maxLenChunkID),
			entity.NewField().WithName(fieldSource).WithDataType(entity.FieldTypeVarChar).WithMaxLength(maxLenSource),
			entity.NewField().WithName(fieldContent).WithDataType(entity.FieldTypeVarChar).WithMaxLength(maxLenContent),
			entity.NewField().WithName(fieldStartLine).WithDataType(entity.FieldTypeInt32),
			entity.NewField().WithName(fieldEndLine).WithDataType(entity.FieldTypeInt32),
			entity.NewField().WithName(fieldChunkType).WithDataType(entity.FieldTypeVarChar).WithMaxLength(maxLenChunkType),
			entity.NewField().WithName(fieldVector).WithDataType(entity.FieldTypeFloatVector).WithDim(int64(vectorDim)),
		},
	}
}

func (s *MilvusStore) createVectorIndex(ctx context.Context) error {
	idx, err := entity.NewIndexHNSW(entity.COSINE, hnswM, hnswEfConstruction)
	if err != nil {
		return fmt.Errorf("milvus: 构建 HNSW 索引参数失败: %w", err)
	}
	if err := s.client.CreateIndex(ctx, s.collection, fieldVector, idx, false); err != nil {
		return fmt.Errorf("milvus: CreateIndex on %q 失败: %w", s.collection, err)
	}
	return nil
}

func validateCollectionSchema(schema *entity.Schema, vectorDim int) error {
	if missing := findMissingFields(schema, requiredFields); len(missing) > 0 {
		return fmt.Errorf("缺少字段 %v", missing)
	}
	actualFields := make(map[string]*entity.Field, len(schema.Fields))
	requiredNames := make(map[string]bool, len(requiredFields))
	for _, name := range requiredFields {
		requiredNames[name] = true
	}
	for _, field := range schema.Fields {
		if field != nil {
			// The v2.4 SDK requires every fixed non-AutoID field in Insert.
			// Reject incompatible schemas before staging a replacement snapshot.
			if !requiredNames[field.Name] && !field.IsDynamic && !field.AutoID {
				return fmt.Errorf("额外必填字段 %s 无法写入源码索引", field.Name)
			}
			actualFields[field.Name] = field
		}
	}
	for _, expected := range codeCollectionSchema("", vectorDim).Fields {
		actual := actualFields[expected.Name]
		if actual.DataType != expected.DataType {
			return fmt.Errorf("字段 %s 类型 %v != %v", expected.Name, actual.DataType, expected.DataType)
		}
		if expected.Name == fieldPK && (!actual.PrimaryKey || !actual.AutoID) {
			return fmt.Errorf("主键 id 必须使用 Int64 AutoID")
		}
		if expected.Name == fieldVector && actual.TypeParams["dim"] != strconv.Itoa(vectorDim) {
			return fmt.Errorf("向量维度 %s != 配置维度 %d", actual.TypeParams["dim"], vectorDim)
		}
		if expected.DataType == entity.FieldTypeVarChar {
			actualMax, err := strconv.Atoi(actual.TypeParams["max_length"])
			expectedMax, _ := strconv.Atoi(expected.TypeParams["max_length"])
			if err != nil || actualMax < expectedMax {
				return fmt.Errorf("字段 %s max_length=%s，至少需要 %d", expected.Name, actual.TypeParams["max_length"], expectedMax)
			}
		}
	}
	return nil
}

// findMissingFields 返回 schema 中缺失的字段名，空表示齐全。
func findMissingFields(schema *entity.Schema, required []string) []string {
	if schema == nil {
		return append([]string{}, required...)
	}
	present := make(map[string]bool, len(schema.Fields))
	for _, f := range schema.Fields {
		if f != nil {
			present[f.Name] = true
		}
	}
	var missing []string
	for _, name := range required {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	return missing
}

// Replace publishes a fully prepared snapshot without deleting the previous data.
func (s *MilvusStore) Replace(ctx context.Context, chunks []CodeChunk, vectors [][]float32) error {
	return s.replaceSnapshot(ctx, chunks, vectors)
}

// Insert 按字节预算分批写入 chunk 与对应向量。
//
// 校验通过后逐批 Insert，所有批次完成后 Flush。调用方仅向未发布快照写入。
func (s *MilvusStore) Insert(ctx context.Context, chunks []CodeChunk, vectors [][]float32) error {
	if len(chunks) == 0 && len(vectors) == 0 {
		return nil
	}
	if err := s.validateChunks(chunks, vectors); err != nil {
		return err
	}
	if s.client == nil {
		return fmt.Errorf("milvus: client 未初始化")
	}

	// A conservative byte budget includes vector payloads, strings and protobuf
	// framing. It stays well below the server's default 64 MiB request limit.
	for start := 0; start < len(chunks); {
		end, size := start, 1024+len(s.collection)
		for end < len(chunks) {
			c := chunks[end]
			rowBytes := len(c.ID) + len(c.Source) + len(c.Content) + len(c.ChunkType) + 4*len(vectors[end]) + 128
			if size+rowBytes > maxInsertBatchBytes {
				break
			}
			size += rowBytes
			end++
		}
		if end == start {
			return fmt.Errorf("milvus: 单条 chunk 超过写入批次限制")
		}
		if err := s.insertBatch(ctx, chunks[start:end], vectors[start:end]); err != nil {
			return err
		}
		start = end
	}
	if err := s.client.Flush(ctx, s.collection, false); err != nil {
		return fmt.Errorf("milvus: flush %q 失败: %w", s.collection, err)
	}
	return nil
}

const maxInsertBatchBytes = 8 << 20

func (s *MilvusStore) insertBatch(ctx context.Context, chunks []CodeChunk, vectors [][]float32) error {
	n := len(chunks)
	chunkIDs := make([]string, n)
	sources := make([]string, n)
	contents := make([]string, n)
	startLines := make([]int32, n)
	endLines := make([]int32, n)
	chunkTypes := make([]string, n)
	for i, c := range chunks {
		chunkIDs[i] = c.ID
		sources[i] = c.Source
		contents[i] = c.Content
		startLines[i] = int32(c.StartLine)
		endLines[i] = int32(c.EndLine)
		chunkTypes[i] = c.ChunkType
	}

	cols := []entity.Column{
		entity.NewColumnVarChar(fieldChunkID, chunkIDs),
		entity.NewColumnVarChar(fieldSource, sources),
		entity.NewColumnVarChar(fieldContent, contents),
		entity.NewColumnInt32(fieldStartLine, startLines),
		entity.NewColumnInt32(fieldEndLine, endLines),
		entity.NewColumnVarChar(fieldChunkType, chunkTypes),
		entity.NewColumnFloatVector(fieldVector, s.vectorDim, vectors),
	}
	if _, err := s.client.Insert(ctx, s.collection, "", cols...); err != nil {
		return fmt.Errorf("milvus: insert %d 条 chunk 失败: %w", n, err)
	}
	return nil
}

func (s *MilvusStore) validateChunks(chunks []CodeChunk, vectors [][]float32) error {
	if len(chunks) != len(vectors) {
		return fmt.Errorf("milvus: insert chunks(%d) 与 vectors(%d) 数量不匹配", len(chunks), len(vectors))
	}
	for i, v := range vectors {
		if len(v) != s.vectorDim {
			return fmt.Errorf("milvus: insert 第 %d 条向量维度 %d != %d", i, len(v), s.vectorDim)
		}
		if err := validateVector(v, s.vectorDim); err != nil {
			return fmt.Errorf("milvus: insert 第 %d 条向量无效: %w", i, err)
		}
	}
	for i, c := range chunks {
		for _, field := range []struct {
			name     string
			value    string
			maxBytes int
		}{
			{fieldChunkID, c.ID, maxLenChunkID},
			{fieldSource, c.Source, maxLenSource},
			{fieldContent, c.Content, maxLenContent},
			{fieldChunkType, c.ChunkType, maxLenChunkType},
		} {
			if strings.TrimSpace(field.value) == "" || len(field.value) > field.maxBytes {
				return fmt.Errorf("milvus: chunk %d 的 %s 不能为空且不可超过 %d 字节", i, field.name, field.maxBytes)
			}
		}
		if c.StartLine < 1 || c.EndLine < c.StartLine || c.EndLine > math.MaxInt32 {
			return fmt.Errorf("milvus: chunk %d 的行号 %d-%d 无效", i, c.StartLine, c.EndLine)
		}
	}
	return nil
}

// Search 在 collection 上执行相似度检索。
//
// - queryVector 为空 / 维度不符 → error
// - TopK <= 0 → 使用 defaultTopK
// - FileFilter 非空 → 使用带字符串转义的 source 精确过滤。
func (s *MilvusStore) Search(ctx context.Context, queryVector []float32, opts SearchOptions) ([]SearchResult, error) {
	if s != nil && s.logical != "" {
		name, _, err := s.existingSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		if name == "" {
			return nil, fmt.Errorf("milvus: collection %q 尚未建立索引，请先运行 index", s.logical)
		}
		// Resolve once per query and use the physical name throughout, including
		// empty-result verification. An alias change cannot mix two generations.
		reader := &MilvusStore{client: s.client, collection: name, vectorDim: s.vectorDim}
		return reader.Search(ctx, queryVector, opts)
	}
	if len(queryVector) == 0 {
		return nil, fmt.Errorf("milvus: search 查询向量为空")
	}
	if len(queryVector) != s.vectorDim {
		return nil, fmt.Errorf("milvus: search 查询向量维度 %d != %d", len(queryVector), s.vectorDim)
	}
	if err := validateVector(queryVector, s.vectorDim); err != nil {
		return nil, fmt.Errorf("milvus: search 查询向量无效: %w", err)
	}
	if err := s.prepareCollection(ctx, false); err != nil {
		return nil, err
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = defaultTopK
	}
	if topK > 16384 {
		return nil, fmt.Errorf("milvus: top_k 不可超过 16384")
	}
	expr := buildFileFilterExpr(opts.FileFilter)

	sp, err := entity.NewIndexHNSWSearchParam(max(hnswSearchEf, topK))
	if err != nil {
		return nil, fmt.Errorf("milvus: 构建搜索参数失败: %w", err)
	}
	outputFields := codeSearchOutputFields()
	rawResults, err := s.client.Search(
		ctx,
		s.collection,
		[]string{},
		expr,
		outputFields,
		[]entity.Vector{entity.FloatVector(queryVector)},
		fieldVector,
		entity.COSINE,
		topK,
		sp,
		client.WithSearchQueryConsistencyLevel(entity.ClStrong),
	)
	if err != nil {
		return nil, fmt.Errorf("milvus: search 失败: %w", err)
	}
	// The legacy SDK can reject either omitted fields or zero-row scalar
	// descriptors returned by Milvus 2.5. Verify that the same filter has no
	// matching entities before treating either known parser error as empty.
	for i, result := range rawResults {
		if !isEmptySearchCompatibilityError(result) {
			continue
		}
		empty, verifyErr := s.confirmEmptySearch(ctx, expr)
		if verifyErr != nil {
			return nil, fmt.Errorf("milvus: 空检索确认失败；原始解析错误: %w；确认错误: %w", result.Err, verifyErr)
		}
		if empty {
			rawResults[i].Err = nil
		}
	}
	return mapMilvusResults(rawResults)
}

func (s *MilvusStore) confirmEmptySearch(ctx context.Context, expression string) (bool, error) {
	if expression == "" {
		expression = fieldPK + " >= 0"
	}
	columns, err := s.client.Query(ctx, s.collection, nil, expression, []string{"count(*)"}, client.WithSearchQueryConsistencyLevel(entity.ClStrong))
	if err != nil {
		return false, err
	}
	if len(columns) != 1 || columns[0] == nil || columns[0].Name() != "count(*)" || columns[0].Type() != entity.FieldTypeInt64 || columns[0].Len() != 1 {
		return false, fmt.Errorf("count(*) 返回了无效结构")
	}
	count, err := columns[0].GetAsInt64(0)
	if err != nil || count < 0 {
		return false, fmt.Errorf("count(*) 返回了无效计数: %v", err)
	}
	return count == 0, nil
}

// Close 关闭底层连接。client 为 nil（未成功构造）时返回 nil，安全幂等。
func (s *MilvusStore) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// buildFileFilterExpr 生成 Milvus expr。空 filter 返回空串（Milvus 允许）。
// 转义反斜杠、双引号及控制字符，避免 expr 语法破坏。
func buildFileFilterExpr(fileFilter string) string {
	trimmed := strings.TrimSpace(fileFilter)
	if trimmed == "" {
		return ""
	}
	return fmt.Sprintf(`%s == %s`, fieldSource, strconv.Quote(trimmed))
}

// mapMilvusResults 将 SDK 的多路 SearchResult 展平成 []SearchResult。
// 每一路若携带 r.Err（SDK 在 parseSearchResult 失败时仍会 append 该路 entry，
// 见 milvus-sdk-go client/data.go），必须返回 error，
// 否则 schema 漂移等根因会被 columnByName 找不到列 -> 零值这条路径静默掩盖。
// 缺字段 / 类型不符时对应字段取零值，不阻断整体映射。
func mapMilvusResults(raw []client.SearchResult) ([]SearchResult, error) {
	var out []SearchResult
	for i, r := range raw {
		if r.Err != nil {
			return nil, fmt.Errorf("milvus: 第 %d 路 search result 解析失败: %w", i, r.Err)
		}
		if r.ResultCount == 0 {
			continue
		}
		chunkIDCol := columnByName(r.Fields, fieldChunkID)
		sourceCol := columnByName(r.Fields, fieldSource)
		contentCol := columnByName(r.Fields, fieldContent)
		startCol := columnByName(r.Fields, fieldStartLine)
		endCol := columnByName(r.Fields, fieldEndLine)
		typeCol := columnByName(r.Fields, fieldChunkType)

		for j := 0; j < r.ResultCount; j++ {
			var score float32
			if j < len(r.Scores) {
				score = r.Scores[j]
			}
			out = append(out, SearchResult{
				ID:        stringAt(chunkIDCol, j),
				Source:    stringAt(sourceCol, j),
				Content:   stringAt(contentCol, j),
				Score:     score,
				StartLine: int(int64At(startCol, j)),
				EndLine:   int(int64At(endCol, j)),
				ChunkType: stringAt(typeCol, j),
			})
		}
	}
	return out, nil
}

func codeSearchOutputFields() []string {
	return []string{fieldChunkID, fieldSource, fieldContent, fieldStartLine, fieldEndLine, fieldChunkType}
}

// Recognize only the two confirmed zero-row parser incompatibilities. A match
// is merely a candidate: Search must additionally confirm count(*) == 0 with
// the same filter and Strong consistency before clearing the parser error.
func isEmptySearchCompatibilityError(result client.SearchResult) bool {
	if result.Err == nil || result.ResultCount != 0 || len(result.Scores) != 0 || len(result.Fields) != 0 || result.GroupByValue != nil {
		return false
	}
	if result.IDs != nil && result.IDs.Len() != 0 {
		return false
	}
	const prefix = "extra output fields ["
	const suffix = "] found and result does not dynamic field"
	message := result.Err.Error()
	if message == "FieldData type not matched" {
		return true
	}
	if !strings.HasPrefix(message, prefix) || !strings.HasSuffix(message, suffix) {
		return false
	}
	fields := strings.Split(strings.TrimSuffix(strings.TrimPrefix(message, prefix), suffix), " ")
	expected := codeSearchOutputFields()
	if len(fields) != len(expected) {
		return false
	}
	remaining := make(map[string]bool, len(expected))
	for _, field := range expected {
		remaining[field] = true
	}
	for _, field := range fields {
		if !remaining[field] {
			return false
		}
		delete(remaining, field)
	}
	return len(remaining) == 0
}

// columnByName 从 ResultSet 中按名字取列，找不到返回 nil。
func columnByName(rs client.ResultSet, name string) entity.Column {
	for _, col := range rs {
		if col.Name() == name {
			return col
		}
	}
	return nil
}

func stringAt(col entity.Column, i int) string {
	if col == nil {
		return ""
	}
	v, err := col.GetAsString(i)
	if err != nil {
		return ""
	}
	return v
}

func int64At(col entity.Column, i int) int64 {
	if col == nil {
		return 0
	}
	v, err := col.GetAsInt64(i)
	if err != nil {
		return 0
	}
	return v
}
