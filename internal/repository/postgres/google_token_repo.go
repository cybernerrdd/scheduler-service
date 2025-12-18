package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type GoogleTokenRepo struct{}

func NewGoogleTokenRepo() *GoogleTokenRepo {
	return &GoogleTokenRepo{}
}

func (r *GoogleTokenRepo) GetTokenByUserID(ctx context.Context, q repository.Querier, userID string) (*models.GoogleToken, error) {
	query := `SELECT id, user_id, access_token, refresh_token, token_type, expiry, created_at, updated_at
		FROM google_tokens
		WHERE user_id = $1`

	var token models.GoogleToken
	err := q.QueryRow(ctx, query, userID).Scan(
		&token.ID,
		&token.UserID,
		&token.AccessToken,
		&token.RefreshToken,
		&token.TokenType,
		&token.Expiry,
		&token.CreatedAt,
		&token.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // Return nil, nil to indicate not found (not an error)
		}
		return nil, err
	}
	return &token, nil
}

func (r *GoogleTokenRepo) SaveToken(ctx context.Context, q repository.Querier, token *models.GoogleToken) error {
	now := time.Now().UTC()
	query := `INSERT INTO google_tokens (id, user_id, access_token, refresh_token, token_type, expiry, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`

	err := q.QueryRow(ctx, query,
		token.UserID,
		token.AccessToken,
		token.RefreshToken,
		token.TokenType,
		token.Expiry,
		now,
		now,
	).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)
	return err
}

func (r *GoogleTokenRepo) UpdateToken(ctx context.Context, q repository.Querier, userID string, token *models.GoogleToken) error {
	now := time.Now().UTC()
	query := `UPDATE google_tokens
		SET access_token = $1, refresh_token = $2, token_type = $3, expiry = $4, updated_at = $5
		WHERE user_id = $6
		RETURNING id, created_at, updated_at`

	err := q.QueryRow(ctx, query,
		token.AccessToken,
		token.RefreshToken,
		token.TokenType,
		token.Expiry,
		now,
		userID,
	).Scan(&token.ID, &token.CreatedAt, &token.UpdatedAt)
	return err
}

