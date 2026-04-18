package tictactoe

func OtherPlayer(players []Player, userID string) *Player {
	for i := range players {
		if players[i].UserID != userID {
			return &players[i]
		}
	}
	return nil
}

func IsWinningBoard(board []string, symbol string) bool {
	for _, l := range WinLines {
		if board[l[0]] == symbol && board[l[1]] == symbol && board[l[2]] == symbol {
			return true
		}
	}
	return false
}
