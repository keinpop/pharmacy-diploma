package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	usecase "pharma/auth/domain/use_case"

	goredis "github.com/redis/go-redis/v9"
)

type SessionRepository struct {
	client *goredis.Client
}

func NewSessionRepository(client *goredis.Client) *SessionRepository {
	return &SessionRepository{client: client}
}

func (r *SessionRepository) Set(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	key := sessionKey(token)
	return r.client.Set(ctx, key, userID, ttl).Err()
}

func (r *SessionRepository) Get(ctx context.Context, token string) (int64, error) {
	key := sessionKey(token)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == goredis.Nil {
			return 0, usecase.ErrInvalidToken
		}
		return 0, fmt.Errorf("redis.SessionRepository.Get: %w", err)
	}

	userID, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("redis.SessionRepository.Get: parse userID: %w", err)
	}
	return userID, nil
}

func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	return r.client.Del(ctx, sessionKey(token)).Err()
}

func sessionKey(token string) string {
	return fmt.Sprintf("session:%s", token)
}
