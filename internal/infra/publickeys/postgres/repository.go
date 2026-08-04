package postgres

import (
	"context"
	"errors"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/merlindorin/sshark-api/internal/domain/publickeys"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*publickeys.Entity, error) {
	var entity publickeys.Entity
	err := r.pool.QueryRow(ctx, `
		SELECT id, source_id, key_type, key_data, provider_key_id, fingerprint, created_at, updated_at
		FROM public_keys
		WHERE id = $1
	`, id).Scan(
		&entity.ID,
		&entity.SourceID,
		&entity.KeyType,
		&entity.KeyData,
		&entity.ProviderKeyID,
		&entity.Fingerprint,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, publickeys.ErrKeyNotFound
		}
		return nil, err
	}

	err = r.loadMetadata(ctx, &entity)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

func (r *Repository) GetByFingerprint(ctx context.Context, fingerprint string) (*publickeys.Entity, error) {
	var entity publickeys.Entity
	err := r.pool.QueryRow(ctx, `
		SELECT id, source_id, key_type, key_data, provider_key_id, fingerprint, created_at, updated_at
		FROM public_keys
		WHERE fingerprint = $1
	`, fingerprint).Scan(
		&entity.ID,
		&entity.SourceID,
		&entity.KeyType,
		&entity.KeyData,
		&entity.ProviderKeyID,
		&entity.Fingerprint,
		&entity.CreatedAt,
		&entity.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, publickeys.ErrKeyNotFound
		}
		return nil, err
	}

	err = r.loadMetadata(ctx, &entity)
	if err != nil {
		return nil, err
	}

	return &entity, nil
}

func (r *Repository) Search(
	ctx context.Context,
	filter publickeys.SearchFilter,
	limit, offset int,
) (*publickeys.SearchResult, error) {
	whereClause, args := buildFilterClause(filter)

	// Count total
	countQuery := `SELECT COUNT(*) FROM public_keys pk` + whereClause
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Build main query with pagination
	argIndex := len(args) + 1
	query := `
		SELECT pk.id, pk.source_id, pk.key_type, pk.key_data, pk.provider_key_id, pk.fingerprint,
		       pk.created_at, pk.updated_at
		FROM public_keys pk` + whereClause
	query += ` ORDER BY pk.created_at DESC LIMIT $` + strconv.Itoa(argIndex)
	query += ` OFFSET $` + strconv.Itoa(argIndex+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities, err := r.scanPublicKeys(ctx, rows)
	if err != nil {
		return nil, err
	}

	return &publickeys.SearchResult{Entities: entities, Total: total}, nil
}

func buildFilterClause(filter publickeys.SearchFilter) (string, []interface{}) {
	whereClause := ` WHERE 1=1`
	var args []interface{}
	argIndex := 1

	if filter.SourceID != nil {
		whereClause += ` AND pk.source_id = $` + strconv.Itoa(argIndex)
		args = append(args, *filter.SourceID)
		argIndex++
	}
	if filter.KeyType != nil {
		whereClause += ` AND pk.key_type = $` + strconv.Itoa(argIndex)
		args = append(args, *filter.KeyType)
		argIndex++
	}
	if filter.Algorithm != nil {
		arg := strconv.Itoa(argIndex)
		whereClause += ` AND EXISTS (
			SELECT 1 FROM ssh_key_metadata sm WHERE sm.key_id = pk.id AND sm.algorithm = $` + arg + `
			UNION
			SELECT 1 FROM gpg_key_metadata gm WHERE gm.key_id = pk.id AND gm.algorithm = $` + arg + `
		)`
		args = append(args, *filter.Algorithm)
	}
	return whereClause, args
}

func (r *Repository) scanPublicKeys(ctx context.Context, rows pgx.Rows) ([]publickeys.Entity, error) {
	var entities []publickeys.Entity
	for rows.Next() {
		var entity publickeys.Entity
		scanErr := rows.Scan(
			&entity.ID,
			&entity.SourceID,
			&entity.KeyType,
			&entity.KeyData,
			&entity.ProviderKeyID,
			&entity.Fingerprint,
			&entity.CreatedAt,
			&entity.UpdatedAt,
		)
		if scanErr != nil {
			return nil, scanErr
		}
		metaErr := r.loadMetadata(ctx, &entity)
		if metaErr != nil {
			return nil, metaErr
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func (r *Repository) SearchWithQuery(
	ctx context.Context,
	keyType publickeys.KeyType,
	whereClause string,
	args []any,
	limit, offset int,
) (*publickeys.SearchResult, error) {
	baseFrom, baseWhere := r.buildSearchBase(keyType)

	fullWhere := baseWhere
	if whereClause != "" {
		fullWhere += " AND " + whereClause
	}

	countQuery := "SELECT COUNT(*) FROM " + baseFrom + fullWhere
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	argIndex := len(args) + 1
	selectQuery := `
		SELECT pk.id, pk.source_id, pk.key_type, pk.key_data, pk.provider_key_id, pk.fingerprint,
		       pk.created_at, pk.updated_at
		FROM ` + baseFrom + fullWhere
	selectQuery += ` ORDER BY pk.created_at DESC LIMIT $` + strconv.Itoa(argIndex)
	selectQuery += ` OFFSET $` + strconv.Itoa(argIndex+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entities, err := r.scanPublicKeys(ctx, rows)
	if err != nil {
		return nil, err
	}

	return &publickeys.SearchResult{Entities: entities, Total: total}, nil
}

func (r *Repository) buildSearchBase(keyType publickeys.KeyType) (string, string) {
	switch keyType {
	case publickeys.KeyTypeSSH:
		return `public_keys pk
			JOIN sources s ON pk.source_id = s.id
			JOIN ssh_key_metadata sm ON pk.id = sm.key_id`,
			" WHERE pk.key_type = 'ssh'"
	case publickeys.KeyTypeGPG:
		return `public_keys pk
			JOIN sources s ON pk.source_id = s.id
			JOIN gpg_key_metadata gm ON pk.id = gm.key_id`,
			" WHERE pk.key_type = 'gpg'"
	default:
		return "public_keys pk JOIN sources s ON pk.source_id = s.id", " WHERE 1=1"
	}
}

func (r *Repository) Create(ctx context.Context, entity *publickeys.Entity) error {
	if entity.ID == uuid.Nil {
		entity.ID = uuid.New()
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO public_keys (id, source_id, key_type, key_data, provider_key_id, fingerprint)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, entity.ID, entity.SourceID, entity.KeyType, entity.KeyData, entity.ProviderKeyID, entity.Fingerprint)
	if err != nil {
		return err
	}

	err = r.saveMetadata(ctx, tx, entity)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) CreateBatch(ctx context.Context, entities []publickeys.Entity) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for i := range entities {
		entity := &entities[i]
		if entity.ID == uuid.Nil {
			entity.ID = uuid.New()
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO public_keys (id, source_id, key_type, key_data, provider_key_id, fingerprint)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, entity.ID, entity.SourceID, entity.KeyType, entity.KeyData, entity.ProviderKeyID, entity.Fingerprint)
		if err != nil {
			return err
		}

		metaErr := r.saveMetadata(ctx, tx, entity)
		if metaErr != nil {
			return metaErr
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) Update(ctx context.Context, entity *publickeys.Entity) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	result, err := tx.Exec(ctx, `
		UPDATE public_keys
		SET key_data = $2, fingerprint = $3, provider_key_id = COALESCE($4, provider_key_id), updated_at = NOW()
		WHERE id = $1
	`, entity.ID, entity.KeyData, entity.Fingerprint, entity.ProviderKeyID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return publickeys.ErrKeyNotFound
	}

	// Delete old metadata and save new
	_, _ = tx.Exec(ctx, `DELETE FROM ssh_key_metadata WHERE key_id = $1`, entity.ID)
	_, _ = tx.Exec(ctx, `DELETE FROM gpg_key_metadata WHERE key_id = $1`, entity.ID)

	err = r.saveMetadata(ctx, tx, entity)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.pool.Exec(ctx, `DELETE FROM public_keys WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return publickeys.ErrKeyNotFound
	}
	return nil
}

func (r *Repository) DeleteBySourceID(ctx context.Context, sourceID uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM public_keys WHERE source_id = $1`, sourceID)
	return err
}

func (r *Repository) ListBySourceID(
	ctx context.Context,
	sourceID uuid.UUID,
	keyType publickeys.KeyType,
) ([]publickeys.Entity, error) {
	query := `
		SELECT pk.id, pk.source_id, pk.key_type, pk.key_data, pk.fingerprint,
		       pk.created_at, pk.updated_at
		FROM public_keys pk
		WHERE pk.source_id = $1`
	args := []any{sourceID}
	if keyType != "" {
		query += ` AND pk.key_type = $2`
		args = append(args, keyType)
	}
	query += ` ORDER BY pk.created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanPublicKeys(ctx, rows)
}

func (r *Repository) AddScrapeHistory(ctx context.Context, history *publickeys.ScrapeHistory) error {
	if history.ID == uuid.Nil {
		history.ID = uuid.New()
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO scrape_history (id, key_id, scraped_at, success, error, key_changed)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, history.ID, history.KeyID, history.ScrapedAt, history.Success, history.Error, history.KeyChanged)
	return err
}

func (r *Repository) GetScrapeHistory(
	ctx context.Context,
	keyID uuid.UUID,
	limit, offset int,
) ([]publickeys.ScrapeHistory, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, key_id, scraped_at, success, error, key_changed
		FROM scrape_history
		WHERE key_id = $1
		ORDER BY scraped_at DESC
		LIMIT $2 OFFSET $3
	`, keyID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []publickeys.ScrapeHistory
	for rows.Next() {
		var h publickeys.ScrapeHistory
		scanErr := rows.Scan(&h.ID, &h.KeyID, &h.ScrapedAt, &h.Success, &h.Error, &h.KeyChanged)
		if scanErr != nil {
			return nil, scanErr
		}
		history = append(history, h)
	}
	return history, nil
}

func (r *Repository) loadMetadata(ctx context.Context, entity *publickeys.Entity) error {
	switch entity.KeyType {
	case publickeys.KeyTypeSSH:
		var meta publickeys.SSHMetadata
		err := r.pool.QueryRow(ctx, `
			SELECT algorithm, comment, options, key_bits
			FROM ssh_key_metadata
			WHERE key_id = $1
		`, entity.ID).Scan(&meta.Algorithm, &meta.Comment, &meta.Options, &meta.KeyBits)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			entity.SSHMetadata = &meta
		}
	case publickeys.KeyTypeGPG:
		var meta publickeys.GPGMetadata
		err := r.pool.QueryRow(ctx, `
			SELECT algorithm, key_bits, expires_at, user_ids, capabilities
			FROM gpg_key_metadata
			WHERE key_id = $1
		`, entity.ID).Scan(&meta.Algorithm, &meta.KeyBits, &meta.ExpiresAt, &meta.UserIDs, &meta.Capabilities)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			entity.GPGMetadata = &meta
		}
	}
	return nil
}

func (r *Repository) saveMetadata(ctx context.Context, tx pgx.Tx, entity *publickeys.Entity) error {
	switch entity.KeyType {
	case publickeys.KeyTypeSSH:
		if entity.SSHMetadata != nil {
			_, err := tx.Exec(ctx, `
				INSERT INTO ssh_key_metadata (key_id, algorithm, comment, options, key_bits)
				VALUES ($1, $2, $3, $4, $5)
			`, entity.ID, entity.SSHMetadata.Algorithm, entity.SSHMetadata.Comment,
				entity.SSHMetadata.Options, entity.SSHMetadata.KeyBits)
			if err != nil {
				return err
			}
		}
	case publickeys.KeyTypeGPG:
		if entity.GPGMetadata != nil {
			_, err := tx.Exec(ctx, `
				INSERT INTO gpg_key_metadata (key_id, algorithm, key_bits, expires_at, user_ids, capabilities)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, entity.ID, entity.GPGMetadata.Algorithm, entity.GPGMetadata.KeyBits,
				entity.GPGMetadata.ExpiresAt, entity.GPGMetadata.UserIDs, entity.GPGMetadata.Capabilities)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
