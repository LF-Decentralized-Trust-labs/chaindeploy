package auth

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- getEncryptionKey tests ---

func TestGetEncryptionKey_WhenSet(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "my-secret-key-123")

	key, err := getEncryptionKey()
	require.NoError(t, err)
	assert.Equal(t, []byte("my-secret-key-123"), key)
}

func TestGetEncryptionKey_WhenMissing(t *testing.T) {
	os.Unsetenv("SESSION_ENCRYPTION_KEY")

	key, err := getEncryptionKey()
	assert.Nil(t, key)
	assert.ErrorIs(t, err, ErrMissingEncryptionKey)
}

func TestGetEncryptionKey_WhenEmpty(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "")

	key, err := getEncryptionKey()
	assert.Nil(t, key)
	assert.ErrorIs(t, err, ErrMissingEncryptionKey)
}

// --- signSessionID tests ---

func TestSignSessionID_Success(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "test-key-for-signing")

	sig, err := signSessionID("session-abc-123")
	require.NoError(t, err)
	assert.NotEmpty(t, sig)
}

func TestSignSessionID_DeterministicOutput(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "deterministic-key")

	sig1, err := signSessionID("same-session")
	require.NoError(t, err)

	sig2, err := signSessionID("same-session")
	require.NoError(t, err)

	assert.Equal(t, sig1, sig2, "same input should produce same signature")
}

func TestSignSessionID_DifferentSessionsDifferentSignatures(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "test-key")

	sig1, err := signSessionID("session-1")
	require.NoError(t, err)

	sig2, err := signSessionID("session-2")
	require.NoError(t, err)

	assert.NotEqual(t, sig1, sig2, "different sessions should produce different signatures")
}

func TestSignSessionID_DifferentKeysDifferentSignatures(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "key-alpha")
	sig1, err := signSessionID("same-session")
	require.NoError(t, err)

	t.Setenv("SESSION_ENCRYPTION_KEY", "key-beta")
	sig2, err := signSessionID("same-session")
	require.NoError(t, err)

	assert.NotEqual(t, sig1, sig2, "different keys should produce different signatures")
}

func TestSignSessionID_FailsWhenKeyMissing(t *testing.T) {
	os.Unsetenv("SESSION_ENCRYPTION_KEY")

	sig, err := signSessionID("any-session")
	assert.Empty(t, sig)
	assert.ErrorIs(t, err, ErrMissingEncryptionKey)
}

func TestSignSessionID_EmptySessionID(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "test-key")

	sig, err := signSessionID("")
	require.NoError(t, err)
	assert.NotEmpty(t, sig, "even empty session ID should produce a signature")
}

// --- verifySessionID tests ---

func TestVerifySessionID_ValidSignature(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "verify-test-key")

	sessionID := "my-session-id"
	sig, err := signSessionID(sessionID)
	require.NoError(t, err)

	assert.True(t, verifySessionID(sessionID, sig))
}

func TestVerifySessionID_InvalidSignature(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "verify-test-key")

	assert.False(t, verifySessionID("my-session-id", "bogus-signature"))
}

func TestVerifySessionID_TamperedSessionID(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "verify-test-key")

	sig, err := signSessionID("original-session")
	require.NoError(t, err)

	// Use valid signature but with a different session ID
	assert.False(t, verifySessionID("tampered-session", sig))
}

func TestVerifySessionID_FailsWhenKeyMissing(t *testing.T) {
	// First sign with key present
	t.Setenv("SESSION_ENCRYPTION_KEY", "key-present")
	sig, err := signSessionID("session-x")
	require.NoError(t, err)

	// Now remove key
	os.Unsetenv("SESSION_ENCRYPTION_KEY")

	// Verification should gracefully return false (not panic)
	assert.False(t, verifySessionID("session-x", sig))
}

func TestVerifySessionID_EmptySignature(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "test-key")
	assert.False(t, verifySessionID("session", ""))
}

func TestVerifySessionID_EmptySessionID(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "test-key")

	sig, err := signSessionID("")
	require.NoError(t, err)

	assert.True(t, verifySessionID("", sig))
}

// --- parseBasicAuth tests ---

func TestParseBasicAuth_ValidCredentials(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("admin", "password123")

	username, password, ok := parseBasicAuth(r)
	assert.True(t, ok)
	assert.Equal(t, "admin", username)
	assert.Equal(t, "password123", password)
}

func TestParseBasicAuth_NoAuthHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)

	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_BearerToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer some-token")

	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_InvalidBase64(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic !!!invalid-base64!!!")

	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_NoColonInPayload(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	// base64("nocolon") = "bm9jb2xvbg=="
	r.Header.Set("Authorization", "Basic bm9jb2xvbg==")

	_, _, ok := parseBasicAuth(r)
	assert.False(t, ok)
}

func TestParseBasicAuth_EmptyPassword(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("user", "")

	username, password, ok := parseBasicAuth(r)
	assert.True(t, ok)
	assert.Equal(t, "user", username)
	assert.Equal(t, "", password)
}

func TestParseBasicAuth_PasswordWithColon(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.SetBasicAuth("user", "pass:word:with:colons")

	username, password, ok := parseBasicAuth(r)
	assert.True(t, ok)
	assert.Equal(t, "user", username)
	assert.Equal(t, "pass:word:with:colons", password)
}

// --- GetSessionID tests ---

func TestGetSessionID_BearerToken(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer my-token-abc")

	sessionID := GetSessionID(r)
	assert.Equal(t, "my-token-abc", sessionID)
}

func TestGetSessionID_ValidCookie(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "cookie-test-key")

	sessionID := "session-from-cookie"
	sig, err := signSessionID(sessionID)
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: sessionID + "." + sig,
	})

	got := GetSessionID(r)
	assert.Equal(t, sessionID, got)
}

func TestGetSessionID_InvalidCookieSignature(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "cookie-test-key")

	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: "session-id.bad-signature",
	})

	got := GetSessionID(r)
	assert.Empty(t, got)
}

func TestGetSessionID_MalformedCookie_NoDot(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: "no-dot-in-value",
	})

	got := GetSessionID(r)
	assert.Empty(t, got)
}

func TestGetSessionID_MalformedCookie_MultipleDots(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: "a.b.c",
	})

	got := GetSessionID(r)
	assert.Empty(t, got)
}

func TestGetSessionID_NoCookieNoHeader(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)

	got := GetSessionID(r)
	assert.Empty(t, got)
}

func TestGetSessionID_BearerTakesPrecedenceOverCookie(t *testing.T) {
	t.Setenv("SESSION_ENCRYPTION_KEY", "test-key")

	sessionID := "cookie-session"
	sig, err := signSessionID(sessionID)
	require.NoError(t, err)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer bearer-token")
	r.AddCookie(&http.Cookie{
		Name:  SessionCookieName,
		Value: sessionID + "." + sig,
	})

	// Bearer should take precedence
	got := GetSessionID(r)
	assert.Equal(t, "bearer-token", got)
}
