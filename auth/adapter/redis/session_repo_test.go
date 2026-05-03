package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	redisrepo "pharma/auth/adapter/redis"
	usecase "pharma/auth/domain/use_case"
)

func newTestRepo(t *testing.T) (*redisrepo.SessionRepository, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	return redisrepo.NewSessionRepository(client), mr
}

// Set ─

func TestSessionRepository_Set(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		token   string
		userID  int64
		ttl     time.Duration
		wantErr bool
	}{
		{name: "short ttl", token: "tok1", userID: 1, ttl: time.Minute},
		{name: "long ttl", token: "tok2", userID: 99, ttl: 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newTestRepo(t)
			err := repo.Set(context.Background(), tt.token, tt.userID, tt.ttl)
			assert.NoError(t, err)
		})
	}
}

// Get ─

func TestSessionRepository_Get(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		setup      func(*redisrepo.SessionRepository)
		token      string
		wantUserID int64
		wantErr    error
	}{
		{
			name: "existing key",
			setup: func(r *redisrepo.SessionRepository) {
				_ = r.Set(context.Background(), "exist", 42, time.Minute)
			},
			token:      "exist",
			wantUserID: 42,
			wantErr:    nil,
		},
		{
			name:    "missing key",
			setup:   func(r *redisrepo.SessionRepository) {},
			token:   "missing",
			wantErr: usecase.ErrInvalidToken,
		},
		{
			name: "expired key",
			setup: func(r *redisrepo.SessionRepository) {
				_ = r.Set(context.Background(), "expired", 7, time.Millisecond)
			},
			token:   "expired",
			wantErr: usecase.ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, mr := newTestRepo(t)
			tt.setup(repo)

			// Имитируем истечение TTL для кейса "expired"
			if tt.name == "expired key" {
				mr.FastForward(time.Second)
			}

			userID, err := repo.Get(context.Background(), tt.token)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Zero(t, userID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantUserID, userID)
			}
		})
	}
}

// Delete ─

func TestSessionRepository_Delete(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(*redisrepo.SessionRepository)
		token string
	}{
		{
			name: "delete existing",
			setup: func(r *redisrepo.SessionRepository) {
				_ = r.Set(context.Background(), "active", 1, time.Minute)
			},
			token: "active",
		},
		{
			name:  "delete nonexistent is ok",
			setup: func(r *redisrepo.SessionRepository) {},
			token: "nonexistent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := newTestRepo(t)
			tt.setup(repo)

			err := repo.Delete(context.Background(), tt.token)
			assert.NoError(t, err)

			// Убеждаемся, что ключ действительно удалён
			_, getErr := repo.Get(context.Background(), tt.token)
			assert.ErrorIs(t, getErr, usecase.ErrInvalidToken)
		})
	}
}
