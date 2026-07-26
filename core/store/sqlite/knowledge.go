package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
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

// knowledgeDocMetaModel replaces the legacy content-in-DB model.
// Body Markdown lives on disk; Content column retained only for migration.
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
	Path      string `gorm:"primaryKey;column:path"`
	KBID      string `gorm:"column:kb_id;index"`
	DocID     string `gorm:"column:doc_id;index"`
	Title     string `gorm:"column:title"`
	Content   string `gorm:"column:content"`
	Embedding string `gorm:"column:embedding"` // JSON []float32
}

func (knowledgeChapterModel) TableName() string { return "knowledge_chapters" }

func (s *Store) KnowledgeBases() port.KnowledgeBaseRepo { return &knowledgeBaseRepo{s} }
func (s *Store) KnowledgeDocs() port.KnowledgeDocRepo   { return &knowledgeDocRepo{s} }
func (s *Store) KnowledgeIndex() port.KnowledgeIndexRepo {
	return &knowledgeIndexRepo{s}
}

func migrateKnowledgeSchema(db *gorm.DB) error {
	if err := rewriteLegacyKnowledgeDocsTable(db); err != nil {
		return err
	}
	if err := db.AutoMigrate(&knowledgeBaseModel{}, &knowledgeDocMetaModel{}, &knowledgeChapterModel{}); err != nil {
		return err
	}
	if err := ensureKnowledgeFTS(db); err != nil {
		return err
	}
	return migrateLegacyKnowledgeDocs(db)
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

func ensureKnowledgeFTS(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	_, err = sqlDB.Exec(`
CREATE VIRTUAL TABLE IF NOT EXISTS knowledge_chapters_fts USING fts5(
  path UNINDEXED,
  title,
  content,
  kb_id UNINDEXED,
  doc_id UNINDEXED,
  tokenize = 'unicode61'
);`)
	return err
}

func migrateLegacyKnowledgeDocs(db *gorm.DB) error {
	// Older schema used autoincrement id + content blob without string id/rel_path.
	// If rows have empty string id, assign ids from rowid.
	type legacy struct {
		RowID   int64  `gorm:"column:rowid"`
		ID      string `gorm:"column:id"`
		KBID    string `gorm:"column:kb_id"`
		Title   string `gorm:"column:title"`
		Content string `gorm:"column:content"`
	}
	var rows []legacy
	// Probe: table may not have rowid alias accessible via gorm; use raw SQL.
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	rs, err := sqlDB.Query(`SELECT rowid, COALESCE(id,''), COALESCE(kb_id,''), COALESCE(title,''), COALESCE(content,'') FROM knowledge_docs`)
	if err != nil {
		return nil // table missing — ignore
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
			_, err := sqlDB.Exec(
				`UPDATE knowledge_docs SET id=?, updated_at=COALESCE(NULLIF(updated_at,''), ?), created_at=COALESCE(NULLIF(created_at,''), ?) WHERE rowid=?`,
				id, now, now, r.RowID,
			)
			if err != nil {
				// column types may already be string PK without rowid update path
				_ = err
			}
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
		ID: row.ID, KBID: row.KBID, Title: row.Title, Path: row.RelPath,
		// Content left empty — service loads from disk (or legacy column via LegacyContent).
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// LegacyDocContent returns leftover DB content for migration to disk (empty if none).
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

type knowledgeIndexRepo struct{ s *Store }

func (r *knowledgeIndexRepo) ReplaceDocChapters(ctx context.Context, docID string, chapters []port.KnowledgeChapter) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if _, err := sqlDB.ExecContext(ctx, `DELETE FROM knowledge_chapters_fts WHERE doc_id = ?`, docID); err != nil {
			return err
		}
		if err := db.WithContext(ctx).Where("doc_id = ?", docID).Delete(&knowledgeChapterModel{}).Error; err != nil {
			return err
		}
		for _, ch := range chapters {
			emb, _ := json.Marshal(ch.Embedding)
			row := knowledgeChapterModel{
				Path: ch.Path, KBID: ch.KBID, DocID: ch.DocID,
				Title: ch.Title, Content: ch.Content, Embedding: string(emb),
			}
			if err := db.WithContext(ctx).Create(&row).Error; err != nil {
				return err
			}
			indexed := kb.CJKBigrams(ch.Title + " " + ch.Content)
			if indexed == "" {
				indexed = ch.Content
			}
			if _, err := sqlDB.ExecContext(ctx,
				`INSERT INTO knowledge_chapters_fts (path, title, content, kb_id, doc_id) VALUES (?,?,?,?,?)`,
				ch.Path, ch.Title, indexed, ch.KBID, ch.DocID,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *knowledgeIndexRepo) DeleteByDoc(ctx context.Context, docID string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if _, err := sqlDB.ExecContext(ctx, `DELETE FROM knowledge_chapters_fts WHERE doc_id = ?`, docID); err != nil {
			return err
		}
		return db.WithContext(ctx).Where("doc_id = ?", docID).Delete(&knowledgeChapterModel{}).Error
	})
}

func (r *knowledgeIndexRepo) DeleteByKB(ctx context.Context, kbID string) error {
	return r.s.withWrite(func(db *gorm.DB) error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		if _, err := sqlDB.ExecContext(ctx, `DELETE FROM knowledge_chapters_fts WHERE kb_id = ?`, kbID); err != nil {
			return err
		}
		return db.WithContext(ctx).Where("kb_id = ?", kbID).Delete(&knowledgeChapterModel{}).Error
	})
}

func (r *knowledgeIndexRepo) SearchBM25(ctx context.Context, kbIDs []string, query string, limit int) ([]domain.KnowledgeChapterHit, error) {
	if limit <= 0 {
		limit = 5
	}
	match := kb.FTSQuery(query)
	if match == "" || len(kbIDs) == 0 {
		return nil, nil
	}
	sqlDB, err := r.s.db.DB()
	if err != nil {
		return nil, err
	}
	placeholders := make([]string, len(kbIDs))
	args := make([]any, 0, len(kbIDs)+2)
	args = append(args, match)
	for i, id := range kbIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, limit)
	q := fmt.Sprintf(`
SELECT f.path, f.kb_id, f.doc_id, c.title, c.content, bm25(knowledge_chapters_fts) AS score
FROM knowledge_chapters_fts f
JOIN knowledge_chapters c ON c.path = f.path
WHERE knowledge_chapters_fts MATCH ?
  AND f.kb_id IN (%s)
ORDER BY score
LIMIT ?`, strings.Join(placeholders, ","))

	rows, err := sqlDB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type hit struct {
		domain.KnowledgeChapterHit
		raw float64
	}
	var hits []hit
	var maxAbs float64
	for rows.Next() {
		var h hit
		var score sql.NullFloat64
		if err := rows.Scan(&h.Path, &h.KBID, &h.DocID, &h.Title, &h.Content, &score); err != nil {
			return nil, err
		}
		// SQLite bm25() is typically negative (more negative = better).
		raw := 0.0
		if score.Valid {
			raw = -score.Float64
		}
		if raw < 0 {
			raw = 0
		}
		h.raw = raw
		if raw > maxAbs {
			maxAbs = raw
		}
		h.Source = "bm25"
		hits = append(hits, h)
	}
	out := make([]domain.KnowledgeChapterHit, 0, len(hits))
	for _, h := range hits {
		if maxAbs > 0 {
			h.Score = h.raw / maxAbs
		} else {
			h.Score = 0
		}
		out = append(out, h.KnowledgeChapterHit)
	}
	return out, nil
}

func (r *knowledgeIndexRepo) ListChapterEmbeddings(ctx context.Context, kbIDs []string) ([]port.KnowledgeChapter, error) {
	if len(kbIDs) == 0 {
		return nil, nil
	}
	var rows []knowledgeChapterModel
	if err := r.s.db.WithContext(ctx).Where("kb_id IN ?", kbIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]port.KnowledgeChapter, 0, len(rows))
	for _, row := range rows {
		var emb []float32
		if row.Embedding != "" {
			_ = json.Unmarshal([]byte(row.Embedding), &emb)
		}
		out = append(out, port.KnowledgeChapter{
			Path: row.Path, KBID: row.KBID, DocID: row.DocID,
			Title: row.Title, Content: row.Content, Embedding: emb,
		})
	}
	return out, nil
}

func (r *knowledgeIndexRepo) SearchVector(ctx context.Context, kbIDs []string, queryVec []float32, limit int) ([]domain.KnowledgeChapterHit, error) {
	if limit <= 0 {
		limit = 5
	}
	chapters, err := r.ListChapterEmbeddings(ctx, kbIDs)
	if err != nil {
		return nil, err
	}
	type scored struct {
		domain.KnowledgeChapterHit
	}
	var hits []scored
	for _, ch := range chapters {
		if len(ch.Embedding) == 0 {
			continue
		}
		sim := kb.CosineSimilarity(queryVec, ch.Embedding)
		hits = append(hits, scored{domain.KnowledgeChapterHit{
			Path: ch.Path, KBID: ch.KBID, DocID: ch.DocID,
			Title: ch.Title, Content: ch.Content, Score: sim, Source: "vector",
		}})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]domain.KnowledgeChapterHit, len(hits))
	for i := range hits {
		out[i] = hits[i].KnowledgeChapterHit
	}
	return out, nil
}
