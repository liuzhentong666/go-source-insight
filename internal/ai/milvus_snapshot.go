package ai

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

// ErrPublicationUnconfirmed means the alias RPC outcome could not be checked.
// Cancellation alone cannot establish that no new snapshot was published.
var ErrPublicationUnconfirmed = errors.New("index publication unconfirmed")

func snapshotKey(logical string) string {
	sum := sha256.Sum256([]byte(logical))
	return hex.EncodeToString(sum[:16])
}

// An internal alias avoids renaming or dropping a pre-existing collection with
// the user-facing name. Older collections remain a read-only fallback until the
// first successful snapshot publication.
func snapshotAlias(logical string) string  { return "gsi_active_" + snapshotKey(logical) }
func snapshotPrefix(logical string) string { return "gsi_snapshot_" + snapshotKey(logical) + "_" }
func snapshotDescription(logical string) string {
	return "GoSource-Insight snapshot: " + snapshotKey(logical)
}

func (s *MilvusStore) logicalName() string {
	if s.logical != "" {
		return s.logical
	}
	return s.collection
}

func physicalCollectionName(collection *entity.Collection) string {
	if collection == nil || collection.Schema == nil {
		return ""
	}
	return collection.Schema.CollectionName
}

// existingSnapshot validates the current schema without changing data, indexes
// or aliases. The boolean reports whether the managed alias already exists.
func (s *MilvusStore) existingSnapshot(ctx context.Context) (string, bool, error) {
	if s == nil || s.client == nil {
		return "", false, fmt.Errorf("milvus: client 未初始化")
	}
	logical := s.logicalName()
	alias := snapshotAlias(logical)
	hasAlias, err := s.client.HasCollection(ctx, alias)
	if err != nil {
		return "", false, fmt.Errorf("milvus: 检查索引入口失败: %w", err)
	}
	name := logical
	if hasAlias {
		name = alias
	} else {
		has, err := s.client.HasCollection(ctx, logical)
		if err != nil {
			return "", false, err
		}
		if !has {
			return "", false, nil
		}
	}
	collection, err := s.client.DescribeCollection(ctx, name)
	if err != nil {
		return "", hasAlias, err
	}
	physical := physicalCollectionName(collection)
	if physical == "" {
		return "", hasAlias, fmt.Errorf("milvus: 索引返回空 collection schema")
	}
	if hasAlias && (!strings.HasPrefix(physical, snapshotPrefix(logical)) || collection.Schema.Description != snapshotDescription(logical)) {
		return "", true, fmt.Errorf("milvus: 内部索引入口名称冲突，拒绝覆盖 %q", alias)
	}
	if err := validateCollectionSchema(collection.Schema, s.vectorDim); err != nil {
		return "", hasAlias, fmt.Errorf("milvus: collection %q schema 不兼容: %w；请选择新的 collection 并重新 index", logical, err)
	}
	return physical, hasAlias, nil
}

func (s *MilvusStore) replaceSnapshot(ctx context.Context, chunks []CodeChunk, vectors [][]float32) (resultErr error) {
	if s == nil || s.client == nil {
		return fmt.Errorf("milvus: client 未初始化")
	}
	if err := s.validateChunks(chunks, vectors); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	previous, hasAlias, err := s.existingSnapshot(ctx)
	if err != nil {
		return err
	}
	logical := s.logicalName()
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("milvus: 生成快照名称失败: %w", err)
	}
	name := snapshotPrefix(logical) + hex.EncodeToString(nonce[:])
	schema := codeCollectionSchema(name, s.vectorDim)
	schema.Description = snapshotDescription(logical)
	if err := s.client.CreateCollection(ctx, schema, entity.DefaultShardNumber); err != nil {
		return fmt.Errorf("milvus: 创建新快照失败，旧索引未修改: %w", err)
	}
	publicationAttempted := false
	defer func() {
		if publicationAttempted {
			// A timeout can happen after the server committed the alias change.
			// Never delete a possibly active snapshot in that ambiguous state.
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := s.client.DropCollection(cleanupCtx, name); err != nil {
			resultErr = fmt.Errorf("%w；清理未发布快照 %q 失败: %w", resultErr, name, err)
		}
	}()
	staged := &MilvusStore{client: s.client, collection: name, vectorDim: s.vectorDim}
	if err := staged.Insert(ctx, chunks, vectors); err != nil {
		return fmt.Errorf("milvus: 新快照写入失败，旧索引未修改: %w", err)
	}
	if err := staged.createVectorIndex(ctx); err != nil {
		return fmt.Errorf("milvus: 新快照建索引失败，旧索引未修改: %w", err)
	}
	if err := s.client.LoadCollection(ctx, name, false); err != nil {
		return fmt.Errorf("milvus: 新快照加载失败，旧索引未修改: %w", err)
	}
	count, err := staged.rowCount(ctx)
	if err != nil {
		return fmt.Errorf("milvus: 新快照校验失败，旧索引未修改: %w", err)
	}
	if count != int64(len(chunks)) {
		return fmt.Errorf("milvus: 新快照条数 %d != %d，旧索引未修改", count, len(chunks))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	publicationAttempted = true
	alias := snapshotAlias(logical)
	if hasAlias {
		err = s.client.AlterAlias(ctx, name, alias)
	} else {
		err = s.client.CreateAlias(ctx, name, alias)
	}
	if err != nil {
		checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		current, checkErr := s.client.DescribeCollection(checkCtx, alias)
		cancel()
		if checkErr != nil || physicalCollectionName(current) != name {
			return fmt.Errorf("%w: milvus: 快照发布未确认，旧数据与新快照 %q 均保留；请检查后重试 index: %w", ErrPublicationUnconfirmed, name, err)
		}
	}
	// Keep historical data on disk for recovery, but do not keep every generation
	// loaded. Indexing the same logical collection must remain single-writer and
	// should not overlap asks that may still be using the previous generation.
	if previous != "" {
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		_ = s.client.ReleaseCollection(releaseCtx, previous)
		cancel()
	}
	s.logical = logical
	s.ready = false
	return nil
}

func (s *MilvusStore) rowCount(ctx context.Context) (int64, error) {
	columns, err := s.client.Query(ctx, s.collection, nil, fieldPK+" >= 0", []string{"count(*)"}, client.WithSearchQueryConsistencyLevel(entity.ClStrong))
	if err != nil {
		return 0, err
	}
	if len(columns) != 1 || columns[0] == nil || columns[0].Name() != "count(*)" || columns[0].Type() != entity.FieldTypeInt64 || columns[0].Len() != 1 {
		return 0, fmt.Errorf("count(*) 返回了无效结构")
	}
	count, err := columns[0].GetAsInt64(0)
	if err != nil || count < 0 {
		return 0, fmt.Errorf("count(*) 返回了无效计数: %v", err)
	}
	return count, nil
}
