package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/oauth2"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type GoogleTokenService struct {
	DB           repository.Querier
	Repo         repository.GoogleTokenRepository
	OAuthConfig  *oauth2.Config
}

func NewGoogleTokenService(db repository.Querier, repo repository.GoogleTokenRepository, oauthConfig *oauth2.Config) *GoogleTokenService {
	return &GoogleTokenService{
		DB:          db,
		Repo:        repo,
		OAuthConfig: oauthConfig,
	}
}

// GetValidToken retrieves a valid access token for the user, refreshing if necessary
// Returns the oauth2.Token ready to use, or an error if tokens don't exist or refresh fails
func (s *GoogleTokenService) GetValidToken(ctx context.Context, userID string) (*oauth2.Token, error) {
	// Get token from DB
	tokenModel, err := s.Repo.GetTokenByUserID(ctx, s.DB, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token from database: %w", err)
	}

	// If no token exists, return error indicating re-login needed
	if tokenModel == nil {
		return nil, errors.New("no Google tokens found for user - re-login required")
	}

	// Convert to oauth2.Token
	token := &oauth2.Token{
		AccessToken:  tokenModel.AccessToken,
		RefreshToken: tokenModel.RefreshToken,
		TokenType:    tokenModel.TokenType,
		Expiry:       tokenModel.Expiry,
	}

	// Check if token is expired (with 5 minute buffer)
	if time.Until(token.Expiry) < 5*time.Minute {
		// Token is expired or about to expire, refresh it
		newToken, err := s.RefreshToken(ctx, userID, token)
		if err != nil {
			return nil, fmt.Errorf("token refresh failed: %w", err)
		}
		return newToken, nil
	}

	// Token is still valid
	return token, nil
}

// RefreshToken refreshes an expired access token using the refresh token
func (s *GoogleTokenService) RefreshToken(ctx context.Context, userID string, oldToken *oauth2.Token) (*oauth2.Token, error) {
	if s.OAuthConfig == nil {
		return nil, errors.New("OAuth config not initialized")
	}

	// Create token source and get new token
	tokenSource := s.OAuthConfig.TokenSource(ctx, oldToken)
	newToken, err := tokenSource.Token()
	if err != nil {
		// Refresh failed - token is invalid/expired, user needs to re-login
		return nil, fmt.Errorf("refresh token expired or invalid: %w", err)
	}

	// Update token in database
	tokenModel := &models.GoogleToken{
		UserID:       userID,
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		TokenType:    newToken.TokenType,
		Expiry:       newToken.Expiry,
	}

	// Check if record exists (update) or create new
	existing, err := s.Repo.GetTokenByUserID(ctx, s.DB, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing token: %w", err)
	}

	if existing != nil {
		// Update existing record
		err = s.Repo.UpdateToken(ctx, s.DB, userID, tokenModel)
		if err != nil {
			return nil, fmt.Errorf("failed to update token in database: %w", err)
		}
	} else {
		// Create new record (shouldn't happen, but handle it)
		err = s.Repo.SaveToken(ctx, s.DB, tokenModel)
		if err != nil {
			return nil, fmt.Errorf("failed to save token in database: %w", err)
		}
	}

	return newToken, nil
}

// SaveToken saves or updates a Google token for a user
func (s *GoogleTokenService) SaveToken(ctx context.Context, userID string, token *oauth2.Token) error {
	tokenModel := &models.GoogleToken{
		UserID:       userID,
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}

	// Check if token already exists
	existing, err := s.Repo.GetTokenByUserID(ctx, s.DB, userID)
	if err != nil {
		return fmt.Errorf("failed to check existing token: %w", err)
	}

	if existing != nil {
		// Update existing record
		err = s.Repo.UpdateToken(ctx, s.DB, userID, tokenModel)
		if err != nil {
			return fmt.Errorf("failed to update token in database: %w", err)
		}
	} else {
		// Create new record
		err = s.Repo.SaveToken(ctx, s.DB, tokenModel)
		if err != nil {
			return fmt.Errorf("failed to save token in database: %w", err)
		}
	}

	return nil
}

// GetTokenModel retrieves the token model from database (for manual refresh endpoint)
func (s *GoogleTokenService) GetTokenModel(ctx context.Context, userID string) (*models.GoogleToken, error) {
	tokenModel, err := s.Repo.GetTokenByUserID(ctx, s.DB, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token from database: %w", err)
	}
	return tokenModel, nil
}

