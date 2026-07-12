package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	redisClient *redis.Client
	useRedis    bool
)

func init() {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisPass := os.Getenv("REDIS_PASSWORD")

	if redisHost != "" {
		port := "6379"
		if redisPort != "" {
			port = redisPort
		}
		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", redisHost, port),
			Password: redisPass,
			DB:       0,
		})
		// Check connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := redisClient.Ping(ctx).Err(); err == nil {
			useRedis = true
			log.Println("[Scheduler] Redis detected. Using Redis Queue for asynchronous session revocation.")
		} else {
			log.Printf("[Scheduler] Redis configured but failed to connect: %v. Falling back to SQL polling.\n", err)
		}
	} else {
		log.Println("[Scheduler] REDIS_HOST not set. Using local SQL polling for revocation.")
	}
}

// ScheduleRevocation registers a session for auto-revocation at the given expiry time.
func ScheduleRevocation(ctx context.Context, sessionID string, expiresAt time.Time) error {
	if !useRedis {
		return nil
	}
	// Add session to sorted set with Unix timestamp score
	err := redisClient.ZAdd(ctx, "session_revocation_queue", redis.Z{
		Score:  float64(expiresAt.Unix()),
		Member: sessionID,
	}).Err()
	if err != nil {
		return fmt.Errorf("failed to schedule revocation in Redis: %w", err)
	}
	log.Printf("[Scheduler] Scheduled session %s for revocation in Redis Queue (TTL: %v)\n", sessionID, time.Until(expiresAt))
	return nil
}

// PopExpiredSessions retrieves and removes expired sessions from the Redis queue.
func PopExpiredSessions(ctx context.Context) ([]string, error) {
	if !useRedis {
		return nil, nil
	}
	now := time.Now().Unix()

	// Get all sessions whose score <= current timestamp
	sessionIDs, err := redisClient.ZRangeByScore(ctx, "session_revocation_queue", &redis.ZRangeBy{
		Min: "-inf",
		Max: fmt.Sprintf("%d", now),
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("failed to range revocation queue: %w", err)
	}

	if len(sessionIDs) > 0 {
		// Remove them from the queue
		err = redisClient.ZRem(ctx, "session_revocation_queue", interfaceSlice(sessionIDs)...).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to remove items from revocation queue: %w", err)
		}
	}

	return sessionIDs, nil
}

func interfaceSlice(slice []string) []interface{} {
	ret := make([]interface{}, len(slice))
	for i, v := range slice {
		ret[i] = v
	}
	return ret
}
