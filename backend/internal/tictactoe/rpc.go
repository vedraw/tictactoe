package tictactoe

import (
	"context"
	"encoding/json"

	"github.com/heroiclabs/nakama-common/api"
)

// MatchRPC is the Nakama subset used by create/list/find match RPCs (testable without a full NakamaModule stub).
type MatchRPC interface {
	MatchCreate(ctx context.Context, module string, params map[string]interface{}) (string, error)
	MatchList(ctx context.Context, limit int, authoritative bool, label string, minSize, maxSize *int, query string) ([]*api.Match, error)
}

// LeaderboardRPC is the Nakama subset used by list_leaderboard.
type LeaderboardRPC interface {
	LeaderboardRecordsList(ctx context.Context, id string, ownerIDs []string, limit int, cursor string, expiry int64) (records []*api.LeaderboardRecord, ownerRecords []*api.LeaderboardRecord, nextCursor string, prevCursor string, err error)
}

func RPCCreateMatch(ctx context.Context, nk MatchRPC, payload string) (string, error) {
	if errObj := checkRPCRateLimit(ctx, "create_match", RateLimitCreateMatch); errObj != nil {
		return MarshalEnvelope(false, nil, errObj), nil
	}
	req, errObj := ParseModePayload(payload)
	if errObj != nil {
		return MarshalEnvelope(false, nil, errObj), nil
	}
	matchID, err := nk.MatchCreate(ctx, "tic_tac_toe", map[string]interface{}{"mode": req.Mode})
	if err != nil {
		return "", err
	}
	return MarshalEnvelope(true, map[string]interface{}{"matchId": matchID, "mode": req.Mode}, nil), nil
}

func RPCListMatches(ctx context.Context, nk MatchRPC, payload string) (string, error) {
	if errObj := checkRPCRateLimit(ctx, "list_matches", RateLimitListMatches); errObj != nil {
		return MarshalEnvelope(false, nil, errObj), nil
	}
	req, errObj := ParseModePayload(payload)
	if errObj != nil {
		return MarshalEnvelope(false, nil, errObj), nil
	}
	authoritative := true
	minSize := 0
	maxSize := 2
	// Use regex OR — parenthesized "(waiting active)" is not reliable in Nakama's match label query parser.
	query := "+label.mode:" + req.Mode + " +label.status:/(waiting|active)/"
	matches, err := nk.MatchList(ctx, 25, authoritative, "", &minSize, &maxSize, query)
	if err != nil {
		return "", err
	}
	return MarshalEnvelope(true, map[string]interface{}{"mode": req.Mode, "matches": matches}, nil), nil
}

func RPCFindMatch(ctx context.Context, nk MatchRPC, payload string) (string, error) {
	if errObj := checkRPCRateLimit(ctx, "find_match", RateLimitFindMatch); errObj != nil {
		return MarshalEnvelope(false, nil, errObj), nil
	}
	req, errObj := ParseModePayload(payload)
	if errObj != nil {
		return MarshalEnvelope(false, nil, errObj), nil
	}
	authoritative := true
	minSize := 0
	maxSize := 1
	query := "+label.mode:" + req.Mode + " +label.status:waiting"
	matches, err := nk.MatchList(ctx, 1, authoritative, "", &minSize, &maxSize, query)
	if err != nil {
		return "", err
	}
	if len(matches) > 0 {
		return MarshalEnvelope(true, map[string]interface{}{
			"matchId": matches[0].MatchId,
			"created": false,
			"mode":    req.Mode,
		}, nil), nil
	}
	matchID, err := nk.MatchCreate(ctx, "tic_tac_toe", map[string]interface{}{"mode": req.Mode})
	if err != nil {
		return "", err
	}
	return MarshalEnvelope(true, map[string]interface{}{"matchId": matchID, "created": true, "mode": req.Mode}, nil), nil
}

func RPCListLeaderboard(ctx context.Context, nk LeaderboardRPC, payload string) (string, error) {
	_ = payload
	records, _, nextCursor, _, err := nk.LeaderboardRecordsList(ctx, LeaderboardID, []string{}, 20, "", 0)
	if err != nil {
		return "", err
	}
	out := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		md := map[string]interface{}{}
		if r.Metadata != "" {
			_ = json.Unmarshal([]byte(r.Metadata), &md)
		}
		uname := ""
		if r.GetUsername() != nil {
			uname = r.GetUsername().GetValue()
		}
		out = append(out, map[string]interface{}{
			"rank":     r.Rank,
			"username": uname,
			"score":    r.Score,
			"wins":     IntFromMetadata(md["wins"]),
			"losses":   IntFromMetadata(md["losses"]),
			"draws":    IntFromMetadata(md["draws"]),
			"streak":   IntFromMetadata(md["streak"]),
		})
	}
	return MarshalEnvelope(true, map[string]interface{}{"leaderboard": out, "cursor": nextCursor}, nil), nil
}
