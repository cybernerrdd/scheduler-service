package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type APIKeyRepo struct{}

func NewAPIKeyRepo() *APIKeyRepo {
	return &APIKeyRepo{}
}

func (r *APIKeyRepo) CreateAPIKey(ctx context.Context, q repository.Querier, email, keyHash string) (*models.APIKey, error) {
	query := `INSERT INTO api_keys (id, email, key_hash, created_at)
		VALUES (gen_random_uuid(), $1, $2, now())
		RETURNING id, email, key_hash, created_at, last_used_at`
	
	var apiKey models.APIKey
	err := q.QueryRow(ctx, query, email, keyHash).Scan(
		&apiKey.ID,
		&apiKey.Email,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (r *APIKeyRepo) GetAPIKeyByHash(ctx context.Context, q repository.Querier, keyHash string) (*models.APIKey, error) {
	query := `SELECT id, email, key_hash, created_at, last_used_at
		FROM api_keys
		WHERE key_hash = $1`
	
	var apiKey models.APIKey
	err := q.QueryRow(ctx, query, keyHash).Scan(
		&apiKey.ID,
		&apiKey.Email,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
	)
	if err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (r *APIKeyRepo) GetAPIKeyByEmail(ctx context.Context, q repository.Querier, email string) (*models.APIKey, error) {
	query := `SELECT id, email, key_hash, created_at, last_used_at
		FROM api_keys
		WHERE email = $1`
	
	var apiKey models.APIKey
	err := q.QueryRow(ctx, query, email).Scan(
		&apiKey.ID,
		&apiKey.Email,
		&apiKey.KeyHash,
		&apiKey.CreatedAt,
		&apiKey.LastUsedAt,
	)
	if err != nil && err != pgx.ErrNoRows {
		return nil, err
	}
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return &apiKey, nil
}

func (r *APIKeyRepo) UpdateAPIKeyHash(ctx context.Context, q repository.Querier, email, keyHash string) error {
	query := `UPDATE api_keys
		SET key_hash = $1
		WHERE email = $2`
	
	_, err := q.Exec(ctx, query, keyHash, email)
	return err
}

func (r *APIKeyRepo) UpdateLastUsed(ctx context.Context, q repository.Querier, keyHash string) error {
	query := `UPDATE api_keys
		SET last_used_at = now()
		WHERE key_hash = $1`
	
	_, err := q.Exec(ctx, query, keyHash)
	return err
}

