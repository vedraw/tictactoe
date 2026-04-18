package tictactoe

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/heroiclabs/nakama-common/runtime"
)

func TestParseModePayload(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		payload string
		want    string
		wantErr string
	}{
		{"empty defaults classic", "", "classic", ""},
		{"classic explicit", `{"mode":"classic"}`, "classic", ""},
		{"timed", `{"mode":"timed"}`, "timed", ""},
		{"empty mode defaults classic", `{"mode":""}`, "classic", ""},
		{"invalid json", `{`, "", ErrorCodes.InvalidPayload},
		{"invalid mode", `{"mode":"blitz"}`, "", ErrorCodes.InvalidMode},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req, errObj := ParseModePayload(tc.payload)
			if tc.wantErr != "" {
				if errObj == nil {
					t.Fatalf("expected error")
				}
				if errObj["code"] != tc.wantErr {
					t.Fatalf("code: got %v want %v", errObj["code"], tc.wantErr)
				}
				return
			}
			if errObj != nil {
				t.Fatalf("unexpected err: %v", errObj)
			}
			if req.Mode != tc.want {
				t.Fatalf("mode: got %q want %q", req.Mode, tc.want)
			}
		})
	}
}

func TestMarshalEnvelope(t *testing.T) {
	t.Parallel()
	ok := MarshalEnvelope(true, map[string]interface{}{"a": 1}, nil)
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(ok), &m); err != nil {
		t.Fatal(err)
	}
	if m["ok"] != true || m["data"].(map[string]interface{})["a"].(float64) != 1 {
		t.Fatalf("success envelope: %v", m)
	}
	errObj := buildError("E", "m", map[string]interface{}{"x": 1})
	s := MarshalEnvelope(false, nil, errObj)
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatal(err)
	}
	if m["ok"] != false {
		t.Fatal("ok false")
	}
	em := m["error"].(map[string]interface{})
	if em["code"] != "E" || em["details"].(map[string]interface{})["x"].(float64) != 1 {
		t.Fatalf("error envelope: %v", em)
	}
}

func TestUserIDFromContext(t *testing.T) {
	t.Parallel()
	if UserIDFromContext(context.Background()) != "anonymous" {
		t.Fatal("missing ctx")
	}
	ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "")
	if UserIDFromContext(ctx) != "anonymous" {
		t.Fatal("empty string")
	}
	ctx = context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "u1")
	if UserIDFromContext(ctx) != "u1" {
		t.Fatal("user id")
	}
}

func TestCheckRPCRateLimit(t *testing.T) {
	t.Run("allows under limit", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "a")
		for i := 0; i < 5; i++ {
			if checkRPCRateLimit(ctx, "create_match", RateLimitCreateMatch) != nil {
				t.Fatalf("call %d throttled", i)
			}
		}
	})
	t.Run("throttles over limit", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "b")
		for i := 0; i < RateLimitCreateMatch; i++ {
			if checkRPCRateLimit(ctx, "create_match", RateLimitCreateMatch) != nil {
				t.Fatalf("unexpected throttle at %d", i)
			}
		}
		if checkRPCRateLimit(ctx, "create_match", RateLimitCreateMatch) == nil {
			t.Fatal("expected throttle on 11th call")
		}
	})
	t.Run("window resets", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "c")
		for i := 0; i < 20; i++ {
			_ = checkRPCRateLimit(ctx, "find_match", RateLimitFindMatch)
		}
		if checkRPCRateLimit(ctx, "find_match", RateLimitFindMatch) == nil {
			t.Fatal("21st should throttle")
		}
		rpcRateMu.Lock()
		rpcRate["find_match:c"] = rateBucket{WindowStart: time.Now().Unix() - RateLimitWindowSec - 1, Count: 0}
		rpcRateMu.Unlock()
		if checkRPCRateLimit(ctx, "find_match", RateLimitFindMatch) != nil {
			t.Fatal("after old window, first call in new window should pass")
		}
	})
	t.Run("per route isolation", func(t *testing.T) {
		ResetRPCRateLimiterForTest()
		ctx := context.WithValue(context.Background(), runtime.RUNTIME_CTX_USER_ID, "d")
		for i := 0; i < RateLimitCreateMatch; i++ {
			_ = checkRPCRateLimit(ctx, "create_match", RateLimitCreateMatch)
		}
		if checkRPCRateLimit(ctx, "list_matches", RateLimitListMatches) != nil {
			t.Fatal("different route should not share counter")
		}
	})
}

func TestIntFromMetadata(t *testing.T) {
	t.Parallel()
	if IntFromMetadata(float64(3)) != 3 || IntFromMetadata(int(4)) != 4 || IntFromMetadata(int64(5)) != 5 {
		t.Fatal("numeric")
	}
	if IntFromMetadata("7") != 7 || IntFromMetadata("x") != 0 {
		t.Fatal("string")
	}
	if IntFromMetadata(nil) != 0 {
		t.Fatal("nil")
	}
}

func TestIsWinningBoard(t *testing.T) {
	t.Parallel()
	b := []string{"", "", "", "", "", "", "", "", ""}
	if IsWinningBoard(b, "X") {
		t.Fatal("empty")
	}
	b[0], b[1], b[2] = "X", "X", "X"
	if !IsWinningBoard(b, "X") || IsWinningBoard(b, "O") {
		t.Fatal("top row X")
	}
	b = []string{"", "", "", "O", "O", "O", "", "", ""}
	if !IsWinningBoard(b, "O") {
		t.Fatal("middle row O")
	}
	b = []string{"X", "O", "X", "O", "X", "O", "", "", "X"}
	if !IsWinningBoard(b, "X") {
		t.Fatal("diag")
	}
}

func TestOtherPlayer(t *testing.T) {
	t.Parallel()
	p := []Player{{UserID: "a"}, {UserID: "b"}}
	if OtherPlayer(p, "a").UserID != "b" {
		t.Fatal("b")
	}
	if OtherPlayer(p, "b").UserID != "a" {
		t.Fatal("a")
	}
	if OtherPlayer([]Player{{UserID: "a"}}, "a") != nil {
		t.Fatal("solo")
	}
	if OtherPlayer(nil, "x") != nil {
		t.Fatal("nil players")
	}
}
