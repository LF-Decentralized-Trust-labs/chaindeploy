package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chainlaunch/chainlaunch/pkg/db"
	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication operations
type AuthService struct {
	db *db.Queries
}

// NewAuthService creates a new authentication service
func NewAuthService(db *db.Queries) *AuthService {
	return &AuthService{
		db: db,
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
	_, err = s.db.CreateUser(context.Background(), &db.CreateUserParams{
		Username: "admin",
		Password: string(hashedPassword),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create user: %w", err)
	}

	return password, nil
}

// GetUser retrieves a user by their ID
func (s *AuthService) GetUser(ctx context.Context, id int64) (*User, error) {
	user, err := s.db.GetUser(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &User{
		ID:       user.ID,
		Username: user.Username,
		Role:     Role(user.Role.String),
	}, nil
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
	dbSession, err := s.db.CreateSession(ctx, &db.CreateSessionParams{
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
		UserID:    user.ID,
		Role:      Role(user.Role.String),
		CreatedAt: dbSession.CreatedAt,
		ExpiresAt: dbSession.ExpiresAt,
	}, nil
}

// ValidateSessionByToken validates a session token and returns the associated user
func (s *AuthService) ValidateSessionByToken(ctx context.Context, token string) (*User, error) {
	// Get session from database
	session, err := s.db.GetSessionByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	// Get user from database
	user, err := s.db.GetUser(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &User{
		ID:       user.ID,
		Username: user.Username,
		Role:     Role(user.Role.String),
	}, nil
}

// ValidateSessionByID validates a session by its session ID and returns the associated user
func (s *AuthService) ValidateSessionByID(ctx context.Context, sessionID string) (*User, error) {
	// Get session from database
	session, err := s.db.GetSessionBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Check if session is expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	// Get user from database
	user, err := s.db.GetUser(ctx, session.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &User{
		ID:       user.ID,
		Username: user.Username,
		Role:     Role(user.Role.String),
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
		ID:          dbUser.ID,
		Username:    dbUser.Username,
		Role:        Role(dbUser.Role.String),
		CreatedAt:   dbUser.CreatedAt,
		LastLoginAt: dbUser.LastLoginAt.Time,
		Password:    dbUser.Password,
	}, nil
}

// Add these new methods to AuthService

// CreateUser creates a new user with the given credentials
func (s *AuthService) CreateUser(ctx context.Context, req *CreateUserRequest) (*User, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	dbUser, err := s.db.CreateUser(ctx, &db.CreateUserParams{
		Username: req.Username,
		Password: string(hashedPassword),
		Role:     sql.NullString{String: string(req.Role), Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &User{
		ID:          dbUser.ID,
		Username:    dbUser.Username,
		Role:        Role(dbUser.Role.String),
		CreatedAt:   dbUser.CreatedAt,
		LastLoginAt: dbUser.LastLoginAt.Time,
	}, nil
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
			Role:        Role(dbUser.Role.String),
			CreatedAt:   dbUser.CreatedAt,
			LastLoginAt: dbUser.LastLoginAt.Time,
		}
	}

	return users, nil
}

// UpdateUser updates a user's details
func (s *AuthService) UpdateUser(ctx context.Context, id int64, req *UpdateUserRequest) (*User, error) {
	// Get existing user
	existingUser, err := s.db.GetUser(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Prepare update params
	params := &db.UpdateUserParams{
		ID:       id,
		Username: existingUser.Username,
		Role:     existingUser.Role,
	}

	// Update fields if provided
	if req.Username != "" {
		params.Username = req.Username
	}
	if req.Role != "" {
		params.Role = sql.NullString{String: string(req.Role), Valid: true}
	}

	// Update user
	dbUser, err := s.db.UpdateUser(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return &User{
		ID:          dbUser.ID,
		Username:    dbUser.Username,
		Role:        Role(dbUser.Role.String),
		CreatedAt:   dbUser.CreatedAt,
		LastLoginAt: dbUser.LastLoginAt.Time,
	}, nil
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

// UpdateUserPassword updates a user's password
func (s *AuthService) UpdateUserPassword(ctx context.Context, username, newPassword string) error {
	// Get user from database
	user, err := s.db.GetUserByUsername(ctx, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Check if new password matches current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(newPassword)); err == nil {
		// Passwords match, no need to update
		return nil
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user's password
	params := &db.UpdateUserPasswordParams{
		ID:       user.ID,
		Password: string(hashedPassword),
	}

	_, err = s.db.UpdateUserPassword(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}

	// Delete all existing sessions for this user for security
	if err := s.db.DeleteUserSessions(ctx, user.ID); err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}
