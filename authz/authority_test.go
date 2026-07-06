package authz

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewServiceAuthorityProvider_RequiresFetch(t *testing.T) {
	_, err := NewServiceAuthorityProvider(nil, nil)
	if !errors.Is(err, ErrServiceAuthorityFetchMissing) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityFetchMissing, err)
	}
}

func TestNewServiceAuthorityProvider_UsesDefaultsWithNilConfig(t *testing.T) {
	provider, err := NewServiceAuthorityProvider(nil, func(context.Context) (*ServiceAuthorityToken, error) {
		return &ServiceAuthorityToken{
			Token:     "service-token",
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}, nil
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	if provider == nil {
		t.Fatalf("expected provider")
	}
}

func TestCachedServiceAuthorityProvider_ReturnsUnavailableBeforeStart(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return &ServiceAuthorityToken{
				Token:     "service-token",
				ExpiresAt: time.Now().Add(time.Hour),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	_, err = provider.ServiceAuthority(context.Background())
	if !errors.Is(err, ErrServiceTokenUnavailable) {
		t.Fatalf("expected %v, got %v", ErrServiceTokenUnavailable, err)
	}
}

func TestCachedServiceAuthorityProvider_StartFetchesTokenInBackground(t *testing.T) {
	var fetchCount int32
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		RefreshBefore: time.Minute,
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			nextFetchCount := atomic.AddInt32(&fetchCount, 1)
			return &ServiceAuthorityToken{
				Token:     "service-token-" + string(rune('0'+nextFetchCount)),
				ExpiresAt: time.Now().Add(10 * time.Minute),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	defer provider.Stop()

	if err := provider.Start(context.Background()); err != nil {
		t.Fatalf("start provider failed: %v", err)
	}

	first, err := waitForServiceAuthority(t, provider)
	if err != nil {
		t.Fatalf("first service authority failed: %v", err)
	}
	second, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("second service authority failed: %v", err)
	}
	if gotFetchCount := atomic.LoadInt32(&fetchCount); first != "service-token-1" || second != first || gotFetchCount != 1 {
		t.Fatalf("expected cached token, first=%q second=%q fetch=%d", first, second, gotFetchCount)
	}

	if refreshed := provider.refreshOnce(context.Background()); !refreshed {
		t.Fatalf("expected manual refresh to succeed")
	}
	third, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("third service authority failed: %v", err)
	}
	if gotFetchCount := atomic.LoadInt32(&fetchCount); third != "service-token-2" || gotFetchCount != 2 {
		t.Fatalf("expected refreshed token, token=%q fetch=%d", third, gotFetchCount)
	}
}

func TestCachedServiceAuthorityProvider_ServiceAuthorityStatusReportsLoadedToken(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		RefreshBefore: 30 * time.Second,
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return &ServiceAuthorityToken{
				Token:     "secret-service-token",
				ExpiresAt: time.Now().Add(time.Hour),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	defer provider.Stop()

	if err := provider.Start(context.Background()); err != nil {
		t.Fatalf("start provider failed: %v", err)
	}
	if _, err := waitForServiceAuthority(t, provider); err != nil {
		t.Fatalf("service authority did not load: %v", err)
	}

	status := provider.ServiceAuthorityStatus()
	if !status.Started || !status.Loaded {
		t.Fatalf("expected started and loaded status, got %+v", status)
	}
	if status.RefreshBefore != "30s" {
		t.Fatalf("unexpected refresh_before: %q", status.RefreshBefore)
	}
	if status.ExpiresAt == nil || status.ExpiresInSeconds <= 0 {
		t.Fatalf("expected future expires_at, got %+v", status)
	}
	body, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status failed: %v", err)
	}
	if strings.Contains(string(body), "secret-service-token") {
		t.Fatalf("status leaked service token: %s", body)
	}
}

func TestCachedServiceAuthorityProvider_RetriesUntilSuccess(t *testing.T) {
	var fetchCount int32
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		RetryBaseInterval: time.Millisecond,
		RetryMaxInterval:  5 * time.Millisecond,
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			nextFetchCount := atomic.AddInt32(&fetchCount, 1)
			if nextFetchCount < 3 {
				return nil, errors.New("auth unavailable")
			}
			return &ServiceAuthorityToken{
				Token:     "service-token",
				ExpiresAt: time.Now().Add(time.Hour),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	defer provider.Stop()

	if err := provider.Start(context.Background()); err != nil {
		t.Fatalf("start provider failed: %v", err)
	}

	token, err := waitForServiceAuthority(t, provider)
	if err != nil {
		t.Fatalf("service authority did not recover: %v", err)
	}
	if gotFetchCount := atomic.LoadInt32(&fetchCount); token != "service-token" || gotFetchCount < 3 {
		t.Fatalf("expected retry success, token=%q fetch=%d", token, gotFetchCount)
	}
}

func TestCachedServiceAuthorityProvider_ServiceAuthorityStatusReportsRefreshError(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return nil, errors.New("auth unavailable")
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	if refreshed := provider.refreshOnce(context.Background()); refreshed {
		t.Fatalf("expected failed refresh")
	}

	status := provider.ServiceAuthorityStatus()
	if status.Loaded {
		t.Fatalf("expected unloaded status, got %+v", status)
	}
	if !strings.Contains(status.LastError, "auth unavailable") {
		t.Fatalf("expected last error, got %+v", status)
	}
}

func TestCachedServiceAuthorityProvider_RejectsCachedTokenWithoutExpiresAt(t *testing.T) {
	fetchCount := 0
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		RefreshBefore: time.Minute,
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			fetchCount++
			return &ServiceAuthorityToken{
				Token:     "service-token",
				ExpiresAt: time.Now().Add(10 * time.Minute),
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	provider.token = "stale-token-without-expires-at"

	if refreshed := provider.refreshOnce(context.Background()); !refreshed {
		t.Fatalf("expected refresh to replace invalid cache")
	}
	token, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("service authority failed: %v", err)
	}
	if token != "service-token" || fetchCount != 1 {
		t.Fatalf("expected zero expires_at cache to be refreshed, token=%q fetch=%d", token, fetchCount)
	}
}

func TestCachedServiceAuthorityProvider_RejectsEmptyToken(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return &ServiceAuthorityToken{}, nil
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	if refreshed := provider.refreshOnce(context.Background()); refreshed {
		t.Fatalf("expected invalid token refresh to fail")
	}
	_, err = provider.ServiceAuthority(context.Background())
	if !errors.Is(err, ErrServiceTokenUnavailable) || !errors.Is(err, ErrServiceAuthorityTokenMissing) {
		t.Fatalf("expected %v and %v, got %v", ErrServiceTokenUnavailable, ErrServiceAuthorityTokenMissing, err)
	}
}

func TestCachedServiceAuthorityProvider_KeepsUnexpiredTokenWhenRefreshFails(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return nil, errors.New("auth unavailable")
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}
	provider.token = "old-service-token"
	provider.expiresAt = time.Now().Add(time.Hour)

	if refreshed := provider.refreshOnce(context.Background()); refreshed {
		t.Fatalf("expected failed refresh")
	}
	token, err := provider.ServiceAuthority(context.Background())
	if err != nil {
		t.Fatalf("expected old token to remain usable: %v", err)
	}
	if token != "old-service-token" {
		t.Fatalf("expected old token, got %q", token)
	}
}

func TestCachedServiceAuthorityProvider_RetryDelayCapsAtMaxInterval(t *testing.T) {
	provider, err := NewCachedServiceAuthorityProvider(CachedServiceAuthorityProviderOptions{
		RetryBaseInterval: time.Minute,
		RetryMaxInterval:  time.Hour,
		Fetch: func(context.Context) (*ServiceAuthorityToken, error) {
			return nil, errors.New("unused")
		},
	})
	if err != nil {
		t.Fatalf("new provider failed: %v", err)
	}

	if got := provider.retryDelay(1); got != 10*time.Minute {
		t.Fatalf("unexpected first retry delay: %v", got)
	}
	if got := provider.retryDelay(6); got != time.Hour {
		t.Fatalf("unexpected capped retry delay: %v", got)
	}
	if got := provider.retryDelay(99); got != time.Hour {
		t.Fatalf("unexpected capped long retry delay: %v", got)
	}
}

func TestNewServiceAuthorityToken_ParsesExpiredValue(t *testing.T) {
	expiresAt := time.Date(2026, 5, 31, 10, 30, 0, 0, time.UTC)
	token, err := NewServiceAuthorityToken(" service-token ", expiresAt.Format(time.RFC3339))
	if err != nil {
		t.Fatalf("new token failed: %v", err)
	}
	if token.Token != "service-token" || !token.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected token: %+v", token)
	}
}

func TestParseServiceAuthorityExpiresAt_RejectsEmptyValue(t *testing.T) {
	_, err := ParseServiceAuthorityExpiresAt("")
	if !errors.Is(err, ErrServiceAuthorityTokenExpiresAtMissing) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityTokenExpiresAtMissing, err)
	}
}

func TestValidateServiceAuthorityToken_RejectsExpiredToken(t *testing.T) {
	err := validateServiceAuthorityToken(&ServiceAuthorityToken{
		Token:     "service-token",
		ExpiresAt: time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC),
	}, time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC))
	if !errors.Is(err, ErrServiceAuthorityTokenExpired) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityTokenExpired, err)
	}
}

func TestValidateServiceAuthorityToken_RejectsMissingExpiresAt(t *testing.T) {
	err := validateServiceAuthorityToken(&ServiceAuthorityToken{Token: "service-token"}, time.Now())
	if !errors.Is(err, ErrServiceAuthorityTokenExpiresAtMissing) {
		t.Fatalf("expected %v, got %v", ErrServiceAuthorityTokenExpiresAtMissing, err)
	}
}

func waitForServiceAuthority(t *testing.T, provider ServiceAuthorityProvider) (string, error) {
	t.Helper()
	deadline := time.Now().Add(300 * time.Millisecond)
	var lastErr error
	for time.Now().Before(deadline) {
		token, err := provider.ServiceAuthority(context.Background())
		if err == nil {
			return token, nil
		}
		lastErr = err
		time.Sleep(time.Millisecond)
	}
	return "", lastErr
}
