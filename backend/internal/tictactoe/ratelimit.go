package tictactoe

import (
	"context"
	"sync"
	"time"

	"github.com/heroiclabs/nakama-common/runtime"
)

var (
	rpcRateMu sync.Mutex
	rpcRate   = map[string]rateBucket{}
)

// ResetRPCRateLimiterForTest clears in-memory RPC rate limit state (tests only).
func ResetRPCRateLimiterForTest() {
	rpcRateMu.Lock()
	rpcRate = map[string]rateBucket{}
	rpcRateMu.Unlock()
}

func checkRPCRateLimit(ctx context.Context, route string, limit int) map[string]interface{} {
	userID := UserIDFromContext(ctx)
	key := route + ":" + userID
	now := time.Now().Unix()
	rpcRateMu.Lock()
	defer rpcRateMu.Unlock()
	b := rpcRate[key]
	if now-b.WindowStart >= RateLimitWindowSec {
		b = rateBucket{WindowStart: now, Count: 0}
	}
	b.Count++
	rpcRate[key] = b
	if b.Count > limit {
		retry := RateLimitWindowSec - int(now-b.WindowStart)
		if retry < 1 {
			retry = 1
		}
		return buildError(ErrorCodes.RateLimitRPC, "RPC rate limit exceeded.", map[string]interface{}{
			"route":         route,
			"limit":         limit,
			"windowSec":     RateLimitWindowSec,
			"retryAfterSec": retry,
		})
	}
	return nil
}

func UserIDFromContext(ctx context.Context) string {
	v := ctx.Value(runtime.RUNTIME_CTX_USER_ID)
	if v == nil {
		return "anonymous"
	}
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return "anonymous"
}
