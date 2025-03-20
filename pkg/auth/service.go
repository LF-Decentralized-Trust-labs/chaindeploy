package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication operations
type AuthService struct {
	db       *db.Queries
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewAuthService creates a new authentication service
func NewAuthService(db *db.Queries) *AuthService {
	return &AuthService{
		db:       db,
		sessions: make(map[string]*Session),
	}
}

// GenerateRandomPassword generates a random password
func GenerateRandomPassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// InitializeDefaultUser creates the default admin user if no users exist
func (s *AuthService) InitializeDefaultUser() (string, error) {
	// Check if any users exist
	count, err := s.db.CountUsers(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to count users: %w", err)
	}

	if count > 0 {
		return "", nil
	}

	// Generate random password
	password, err := GenerateRandomPassword(12)
	if err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	// Create admin user
	_, err = s.db.CreateUser(context.Background(), db.CreateUserParams{
		Username: "admin",
		Password: string(hashedPassword),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return password, nil
}

// Login authenticates a user and returns a session
func (s *AuthService) Login(ctx context.Context, username, password string) (*Session, error) {
	// Get user from database
	user, err := s.db.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Generate session ID
	sessionID := make([]byte, 32)
	if _, err := rand.Read(sessionID); err != nil {
		return nil, fmt.Errorf("failed to generate session ID: %w", err)
	}
	id := base64.URLEncoding.EncodeToString(sessionID)

	// Generate token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Create session in database
	expiresAt := time.Now().Add(24 * time.Hour) // Sessions expire after 24 hours
	dbSession, err := s.db.CreateSession(ctx, db.CreateSessionParams{
		SessionID: id,
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Update last login time
	_, err = s.db.UpdateUserLastLogin(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to update last login: %w", err)
	}

	return &Session{
		ID:        dbSession.SessionID,
		Token:     token,
		Username:  username,
		CreatedAt: dbSession.CreatedAt,
		ExpiresAt: dbSession.ExpiresAt,
	}, nil
}

// ValidateSession checks if a session is valid
func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (*Session, error) {
	dbSession, err := s.db.GetSession(ctx, sessionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &Session{
		ID:        dbSession.SessionID,
		Username:  dbSession.Username,
		CreatedAt: dbSession.CreatedAt,
		ExpiresAt: dbSession.ExpiresAt,
	}, nil
}

// Logout removes a session
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.db.DeleteSession(ctx, sessionID)
}

// CleanupExpiredSessions removes all expired sessions
func (s *AuthService) CleanupExpiredSessions(ctx context.Context) error {
	return s.db.DeleteExpiredSessions(ctx)
}

// LogoutAllUserSessions removes all sessions for a user
func (s *AuthService) LogoutAllUserSessions(ctx context.Context, userID int64) error {
	return s.db.DeleteUserSessions(ctx, userID)
}

// GetUserByUsername retrieves a user by username
func (s *AuthService) GetUserByUsername(username string) (*User, error) {
	dbUser, err := s.db.GetUserByUsername(context.Background(), username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &User{
		Username:    dbUser.Username,
		CreatedAt:   dbUser.CreatedAt,
		LastLoginAt: dbUser.LastLoginAt.Time,
	}, nil
}

// Add these new types at the top of the file after the imports
type CreateUserRequest struct {
	Username string
	Password string
}

// Add these new methods to AuthService

// CreateUser creates a new user with the given credentials
func (s *AuthService) CreateUser(ctx context.Context, username, password string) error {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	_, err = s.db.CreateUser(ctx, db.CreateUserParams{
		Username: username,
		Password: string(hashedPassword),
	})
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// ListUsers returns all users
func (s *AuthService) ListUsers(ctx context.Context) ([]*User, error) {
	dbUsers, err := s.db.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	users := make([]*User, len(dbUsers))
	for i, dbUser := range dbUsers {
		users[i] = &User{
			ID:          dbUser.ID,
			Username:    dbUser.Username,
			CreatedAt:   dbUser.CreatedAt,
			LastLoginAt: dbUser.LastLoginAt.Time,
		}
	}

	return users, nil
}

// UpdateUser updates a user's details
func (s *AuthService) UpdateUser(ctx context.Context, id int64, username, password string) error {
	params := db.UpdateUserParams{
		ID:       id,
		Username: username,
	}

	if password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		params.Password = string(hashedPassword)
	}

	_, err := s.db.UpdateUser(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// DeleteUser deletes a user by ID
func (s *AuthService) DeleteUser(ctx context.Context, id int64) error {
	if err := s.db.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, id int64) (*User, error) {
	dbUser, err := s.db.GetUser(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &User{
		ID:          dbUser.ID,
		Username:    dbUser.Username,
		CreatedAt:   dbUser.CreatedAt,
		LastLoginAt: dbUser.LastLoginAt.Time,
	}, nil
}
