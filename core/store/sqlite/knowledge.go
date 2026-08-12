package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"danmo-work/core/domain"
	"danmo-work/core/port"
	kb "danmo-work/core/runtime/knowledge"

	"gorm.io/gorm"
)

type knowledgeBaseModel struct {
	ID          string `gorm:"primaryKey;column:id"`
	Name        string `gorm:"column:name"`
	Description string `gorm:"column:description"`
	CreatedAt   string `gorm:"column:created_at"`
	UpdatedAt   string `gorm:"column:updated_at"`
}

func (knowledgeBaseModel) TableName() string { return "knowledge_bases" }

type knowledgeDocMetaModel struct {
	ID        string `gorm:"primaryKey;column:id"`
	KBID      string `gorm:"column:kb_id;index"`
	Title     string `gorm:"column:title"`
	RelPath   string `gorm:"column:rel_path"`
	Content   string `gorm:"column:content"` // legacy; cleared after disk migration
	CreatedAt string `gorm:"column:created_at"`
	UpdatedAt string `gorm:"column:updated_at"`
}

func (knowledgeDocMetaModel) TableName() string { return "knowledge_docs" }

type knowledgeChapterModel struct {
	ID      string `gorm:"primaryKey;column:id"`
	KBID    string `gorm:"column:kb_id;index"`
	DocID   string `gorm:"column:doc_id;index"`
	Title   string `gorm:"column:title"`
	Content string `gorm:"column:content"` // full chapter Markdown
}

func (knowledgeChapterModel) TableName() string { return "knowledge_chapters" }

func (s *Store) KnowledgeBases() port.KnowledgeBaseRepo  { return &knowledgeBaseRepo{s} }
func (s *Store) KnowledgeDocs() port.KnowledgeDocRepo     { return &knowledgeDocRepo{s} }
func (s *Store) KnowledgeIndex() port.KnowledgeIndexRepo  { return &knowledgeIndexRepo{s, kb.NewInvertedIndex(), make(map[string][]float32)} }

func migrateKnowledgeSchema(db *gorm.DB) error {
	if err := rewriteLegacyKnowledgeDocsTable(db); err != nil {
		return err
	}
	if err := resetBrokenKnowledgeChapters(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(&knowledgeBaseModel{}, &knowledgeDocMetaModel{}, &knowledgeChapterModel{}); err != nil {
		return err
	}
	_ = dropOldKnowledgeChunks(db)
	return migrateLegacyKnowledgeDocs(db)
}

func resetBrokenKnowledgeChapters(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	_, _ = sqlDB.Exec(`DROP TABLE IF EXISTS knowledge_chapters_legacy`)
	_, _ = sqlDB.Exec(`DROP TABLE IF EXISTS knowledge_chapters_fts`)
	// Drop old v1 schema: PK=path, has embedding column; recreate with id PK.
	if db.Migrator().HasTable("knowledge_chapters") {
		rows, _ := sqlDB.Query(`PRAGMA table_info(knowledge_chapters)`)
		if rows != nil {
			defer rows.Close()
			hasPathPK := false
			hasID := false
			for rows.Next() {
				var cid int
				var name, typ string
				var notnull, pk int
				var dflt sql.NullString
				_ = rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk)
				lower := strings.ToLower(name)
				if lower == "path" && pk > 0 {
					hasPathPK = true
				}
				if lower == "id" {
					hasID = true
				}
			}
			if hasPathPK && !hasID {
				_, _ = sqlDB.Exec(`DROP TABLE IF EXISTS knowledge_chapters`)
			}
		}
	}
	return nil
}

func dropOldKnowledgeChunks(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	_, _ = sqlDB.Exec(`DROP TABLE IF EXISTS knowledge_chunks`)
	_, _ = sqlDB.Exec(`DROP TABLE IF EXISTS knowledge_chunks_fts`)
	return nil
}

func rewriteLegacyKnowledgeDocsTable(db *gorm.DB) error {
	if !db.Migrator().HasTable("knowledge_docs") {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	rows, err := sqlDB.Query(`PRAGMA table_info(knowledge_docs)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	hasStringID := false
	hasRelPath := false
	idIsInteger := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return err
		}
		lower := strings.ToLower(name)
		typeLower := strings.ToLower(typ)
		if lower == "id" {
			if strings.Contains(typeLower, "int") {
				idIsInteger = true
			} else {
				hasStringID = true
			}
		}
		if lower == "rel_path" {
			hasRelPath = true
		}
	}
	if hasStringID && hasRelPath && !idIsInteger {
		return nil
	}
	// Rebuild table: integer-PK legacy → string-PK + rel_path.
	if _, err := sqlDB.Exec(`ALTER TABLE knowledge_docs RENAME TO knowledge_docs_legacy`); err != nil {
		return err
	}
	if err := db.AutoMigrate(&knowledgeDocMetaModel{}); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = sqlDB.Exec(`
INSERT INTO knowledge_docs (id, kb_id, title, rel_path, content, created_at, updated_at)
SELECT
  'doc-legacy-' || rowid,
  COALESCE(kb_id, ''),
  COALESCE(title, ''),
  '',
  COALESCE(content, ''),
  ?,
  ?
FROM knowledge_docs_legacy`, now, now)
	if err != nil {
		return fmt.Errorf("migrate legacy knowledge_docs: %w", err)
	}
	_, _ = sqlDB.Exec(`DROP TABLE knowledge_docs_legacy`)
	return nil
}

func migrateLegacyKnowledgeDocs(db *gorm.DB) error {
	type legacy struct {
		RowID   int64  `gorm:"column:rowid"`
		ID      string `gorm:"column:id"`
		KBID    string `gorm:"column:kb_id"`
		Title   string `gorm:"column:title"`
		Content string `gorm:"column:content"`
	}
	var rows []legacy
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	rs, err := sqlDB.Query(`SELECT rowid, COALESCE(id,''), COALESCE(kb_id,''), COALESCE(title,''), COALESCE(content,'') FROM knowledge_docs`)
	if err != nil {
		return nil
	}
	defer rs.Close()
	for rs.Next() {
		var r legacy
		if err := rs.Scan(&r.RowID, &r.ID, &r.KBID, &r.Title, &r.Content); err != nil {
			return err
		}
		rows = append(rows, r)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range rows {
		id := strings.TrimSpace(r.ID)
		if id == "" || id == "0" {
			id = fmt.Sprintf("doc-legacy-%d", r.RowID)
			_, _ = sqlDB.Exec(
				`UPDATE knowledge_docs SET id=?, updated_at=COALESCE(NULLIF(updated_at,''), ?), created_at=COALESCE(NULLIF(created_at,''), ?) WHERE rowid=?`,
				id, now, now, r.RowID,
			)
		}
		if r.KBID != "" {
			var n int
			_ = sqlDB.QueryRow(`SELECT COUNT(1) FROM knowledge_bases WHERE id=?`, r.KBID).Scan(&n)
			if n == 0 {
				_, _ = sqlDB.Exec(
					`INSERT OR IGNORE INTO knowledge_bases (id, name, description, created_at, updated_at) VALUES (?,?,?,?,?)`,
					r.KBID, r.KBID, "", now, now,
				)
			}
		}
	}
	return nil
}

// ---- KnowledgeBaseRepo ----

type knowledgeBaseRepo struct{ s *Store }

func (r *knowledgeBaseRepo) List(ctx context.Context) ([]domain.KnowledgeBase, error) {
	var rows []knowledgeBaseModel
	if err := r.s.db.WithContext(ctx).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.KnowledgeBase, len(rows))
	for i, row := range rows {
		out[i] = baseToDomain(row)
		n, _ := r.s.KnowledgeDocs().CountByKB(ctx, row.ID)
		out[i].DocumentCount = n
	}
	return out, nil
}

func (r *knowledgeBaseRepo) Get(ctx context.Context, id string) (domain.KnowledgeBase, error) {
	var row knowledgeBaseModel
	err := r.s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.KnowledgeBase{}, fmt.Errorf("knowledge base not found")
	}
	if err != nil {
		return domain.KnowledgeBase{}, err
	}
	b := baseToDomain(row)
	b.DocumentCount, _ = r.s.KnowledgeDocs().CountByKB(ctx, id)
	return b, nil
}

func (r *knowledgeBaseRepo) Upsert(ctx context.Context, b domain.KnowledgeBase) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		row := knowledgeBaseModel{
			ID: b.ID, Name: b.Name, Description: b.Description,
			CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
		}
		return db.WithContext(ctx).Save(&row).Error
	})
}

func (r *knowledgeBaseRepo) Delete(ctx context.Context, id string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("id = ?", id).Delete(&knowledgeBaseModel{}).Error
	})
}

func baseToDomain(row knowledgeBaseModel) domain.KnowledgeBase {
	return domain.KnowledgeBase{
		ID: row.ID, Name: row.Name, Description: row.Description,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// ---- KnowledgeDocRepo ----

type knowledgeDocRepo struct{ s *Store }

func (r *knowledgeDocRepo) ListByKB(ctx context.Context, kbID string) ([]domain.KnowledgeDoc, error) {
	var rows []knowledgeDocMetaModel
	if err := r.s.db.WithContext(ctx).Where("kb_id = ?", kbID).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return docsToDomain(rows), nil
}

func (r *knowledgeDocRepo) ListAll(ctx context.Context) ([]domain.KnowledgeDoc, error) {
	var rows []knowledgeDocMetaModel
	if err := r.s.db.WithContext(ctx).Order("updated_at desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return docsToDomain(rows), nil
}

func (r *knowledgeDocRepo) Get(ctx context.Context, id string) (domain.KnowledgeDoc, error) {
	var row knowledgeDocMetaModel
	err := r.s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.KnowledgeDoc{}, fmt.Errorf("knowledge doc not found")
	}
	if err != nil {
		return domain.KnowledgeDoc{}, err
	}
	return docToDomain(row), nil
}

func (r *knowledgeDocRepo) Upsert(ctx context.Context, d domain.KnowledgeDoc) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		row := knowledgeDocMetaModel{
			ID: d.ID, KBID: d.KBID, Title: d.Title, RelPath: d.Path,
			CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
		}
		return db.WithContext(ctx).Save(&row).Error
	})
}

func (r *knowledgeDocRepo) Delete(ctx context.Context, id string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("id = ?", id).Delete(&knowledgeDocMetaModel{}).Error
	})
}

func (r *knowledgeDocRepo) DeleteByKB(ctx context.Context, kbID string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("kb_id = ?", kbID).Delete(&knowledgeDocMetaModel{}).Error
	})
}

func (r *knowledgeDocRepo) CountByKB(ctx context.Context, kbID string) (int, error) {
	var n int64
	err := r.s.db.WithContext(ctx).Model(&knowledgeDocMetaModel{}).Where("kb_id = ?", kbID).Count(&n).Error
	return int(n), err
}

func docsToDomain(rows []knowledgeDocMetaModel) []domain.KnowledgeDoc {
	out := make([]domain.KnowledgeDoc, len(rows))
	for i, row := range rows {
		out[i] = docToDomain(row)
	}
	return out
}

func docToDomain(row knowledgeDocMetaModel) domain.KnowledgeDoc {
	return domain.KnowledgeDoc{
		ID:        row.ID, KBID: row.KBID, Title: row.Title, Path: row.RelPath,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func (s *Store) LegacyDocContent(ctx context.Context, id string) (string, error) {
	var row knowledgeDocMetaModel
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if err != nil {
		return "", err
	}
	return row.Content, nil
}

func (s *Store) ClearLegacyDocContent(ctx context.Context, id string) error {
	return s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Model(&knowledgeDocMetaModel{}).Where("id = ?", id).Update("content", "").Error
	})
}

// ---- KnowledgeIndexRepo ----

type knowledgeIndexRepo struct {
	s        *Store
	inverted *kb.InvertedIndex
	vectors  map[string][]float32 // chunkID → embedding
}

func (r *knowledgeIndexRepo) ReplaceDocChapters(ctx context.Context, docID string, chapters []port.KnowledgeChapter, chunks []port.KnowledgeChunkEntry, vectors map[string][]float32) error {
	// 1. Persist chapters to the knowledge_chapters table.
	if err := r.s.withWrite(func(db *gorm.DB) error {
		if err := db.WithContext(ctx).Where("doc_id = ?", docID).Delete(&knowledgeChapterModel{}).Error; err != nil {
			return err
		}
		for _, ch := range chapters {
			row := knowledgeChapterModel{
				ID: ch.ID, KBID: ch.KBID, DocID: ch.DocID,
				Title: ch.Title, Content: ch.Content,
			}
			if err := db.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 2. Update in-memory BM25 inverted index.
	entries := make([]kb.IndexEntry, len(chunks))
	for i, c := range chunks {
		entries[i] = kb.IndexEntry{ID: c.ID, KBID: c.KBID, DocID: c.DocID, Text: c.Text}
	}
	r.inverted.Index(docID, entries)

	// 3. Update in-memory vector index.
	for cid, vec := range vectors {
		r.vectors[cid] = vec
	}
	return nil
}

func (r *knowledgeIndexRepo) DeleteByDoc(ctx context.Context, docID string) error {
	if err := r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("doc_id = ?", docID).Delete(&knowledgeChapterModel{}).Error
	}); err != nil {
		return err
	}
	r.inverted.DeleteByDoc(docID)
	return nil
}

func (r *knowledgeIndexRepo) DeleteByKB(ctx context.Context, kbID string) error {
	if err := r.s.withWrite(func(db *gorm.DB) error {
		return db.WithContext(ctx).Where("kb_id = ?", kbID).Delete(&knowledgeChapterModel{}).Error
	}); err != nil {
		return err
	}
	r.inverted.DeleteByKB(kbID)
	return nil
}

func (r *knowledgeIndexRepo) SearchBM25(ctx context.Context, kbIDs []string, query string, limit int) ([]domain.KnowledgeChunkHit, error) {
	_ = ctx
	hits := r.inverted.Search(query, kbIDs, limit)
	out := make([]domain.KnowledgeChunkHit, len(hits))
	for i, h := range hits {
		out[i] = domain.KnowledgeChunkHit{ID: h.ChunkID, Score: h.Score, Source: "bm25"}
	}
	return out, nil
}

func (r *knowledgeIndexRepo) SearchVector(ctx context.Context, kbIDs []string, queryVec []float32, limit int) ([]domain.KnowledgeChunkHit, error) {
	_ = ctx
	if limit <= 0 {
		limit = 5
	}
	if len(queryVec) == 0 || len(kbIDs) == 0 {
		return nil, nil
	}

	// Get chunk metadata from inverted index for KB filtering.
	byKB := r.inverted.ChunkIDsByKB(kbIDs)
	if len(byKB) == 0 {
		return nil, nil
	}

	type scored struct {
		domain.KnowledgeChunkHit
	}
	var hits []scored
	for cid := range byKB {
		vec, ok := r.vectors[cid]
		if !ok || len(vec) == 0 {
			continue
		}
		sim := kb.CosineSimilarity(queryVec, vec)
		hits = append(hits, scored{domain.KnowledgeChunkHit{
			ID: cid, Score: sim, Source: "vector",
		}})
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]domain.KnowledgeChunkHit, len(hits))
	for i := range hits {
		out[i] = hits[i].KnowledgeChunkHit
	}
	return out, nil
}

func (r *knowledgeIndexRepo) GetChaptersByIDs(ctx context.Context, chapterIDs []string) ([]domain.KnowledgeChapter, error) {
	if len(chapterIDs) == 0 {
		return nil, nil
	}
	var rows []knowledgeChapterModel
	if err := r.s.db.WithContext(ctx).Where("id IN ?", chapterIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.KnowledgeChapter, len(rows))
	for i, row := range rows {
		out[i] = domain.KnowledgeChapter{
			ID: row.ID, KBID: row.KBID, DocID: row.DocID,
			Title: row.Title, Content: row.Content,
		}
	}
	return out, nil
}
