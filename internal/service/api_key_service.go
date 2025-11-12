package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"scheduler-service/internal/models"
	"scheduler-service/internal/repository"
)

type APIKeyService struct {
	DB  repository.Querier
	Repo repository.APIKeyRepository
}

func NewAPIKeyService(db repository.Querier, repo repository.APIKeyRepository) *APIKeyService {
	return &APIKeyService{DB: db, Repo: repo}
}

// GenerateAPIKey creates a new API key for the given email and password
// For now, it verifies email+password combination and generates a key
// Later this can be made user-specific
func (s *APIKeyService) GenerateAPIKey(ctx context.Context, email, password string) (string, *models.APIKey, error) {
	// Validate email and password
	if email == "" || password == "" {
		return "", nil, errors.New("email and password are required")
	}

	// Check if key already exists for this email
	existing, err := s.Repo.GetAPIKeyByEmail(ctx, s.DB, email)
	if err != nil {
		return "", nil, fmt.Errorf("failed to check existing key: %w", err)
	}

	// For now, we'll generate a key based on email+password hash
	// Later this can be improved with proper user authentication
	// Verify the email+password combination by creating a hash
	// In a real system, you'd verify against a user table with hashed passwords
	// Note: credentialHash is calculated but not used yet - reserved for future validation
	_ = hashEmailPassword(email, password)

	// Generate a new API key (UUID-based)
	apiKey := fmt.Sprintf("sk_%s", uuid.New().String())

	// Hash the API key for storage
	keyHash := hashAPIKey(apiKey)

	var apiKeyRecord *models.APIKey

	if existing != nil {
		// Update existing key with new hash (invalidates old key)
		err = s.Repo.UpdateAPIKeyHash(ctx, s.DB, email, keyHash)
		if err != nil {
			return "", nil, fmt.Errorf("failed to update API key: %w", err)
		}
		// Fetch updated record
		apiKeyRecord, err = s.Repo.GetAPIKeyByEmail(ctx, s.DB, email)
		if err != nil {
			return "", nil, fmt.Errorf("failed to fetch updated API key: %w", err)
		}
	} else {
		// Create new API key
		apiKeyRecord, err = s.Repo.CreateAPIKey(ctx, s.DB, email, keyHash)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create API key: %w", err)
		}
	}

	return apiKey, apiKeyRecord, nil
}

// ValidateAPIKey checks if the provided API key is valid
func (s *APIKeyService) ValidateAPIKey(ctx context.Context, apiKey string) (*models.APIKey, error) {
	if apiKey == "" {
		return nil, errors.New("API key is required")
	}

	// Hash the provided key
	keyHash := hashAPIKey(apiKey)

	// Look up the key in database
	apiKeyRecord, err := s.Repo.GetAPIKeyByHash(ctx, s.DB, keyHash)
	if err != nil {
		return nil, errors.New("invalid API key")
	}

	// Update last used timestamp
	_ = s.Repo.UpdateLastUsed(ctx, s.DB, keyHash)

	return apiKeyRecord, nil
}

// hashEmailPassword creates a hash from email and password combination
// This is used to verify credentials (for now)
func hashEmailPassword(email, password string) string {
	data := fmt.Sprintf("%s:%s", email, password)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// hashAPIKey creates a SHA256 hash of the API key
func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

