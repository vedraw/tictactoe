package main

import (
	"context"
	"database/sql"

	"tictactoe-backend/internal/tictactoe"

	"github.com/heroiclabs/nakama-common/runtime"
)

func InitModule(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, initializer runtime.Initializer) error {
	_ = nk.LeaderboardCreate(ctx, tictactoe.LeaderboardID, true, "desc", "best", "", map[string]interface{}{})
	if err := initializer.RegisterMatch("tic_tac_toe", tictactoe.NewMatch); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("create_match", rpcCreateMatch); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("list_matches", rpcListMatches); err != nil {
		return err
	}
	if err := initializer.RegisterRpc("find_match", rpcFindMatch); err != nil {
		return err
	}
	return initializer.RegisterRpc("list_leaderboard", rpcListLeaderboard)
}

func rpcCreateMatch(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	return tictactoe.RPCCreateMatch(ctx, nk, payload)
}

func rpcListMatches(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	return tictactoe.RPCListMatches(ctx, nk, payload)
}

func rpcFindMatch(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	return tictactoe.RPCFindMatch(ctx, nk, payload)
}

func rpcListLeaderboard(ctx context.Context, logger runtime.Logger, db *sql.DB, nk runtime.NakamaModule, payload string) (string, error) {
	return tictactoe.RPCListLeaderboard(ctx, nk, payload)
}
