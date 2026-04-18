package tictactoe

const (
	BoardSize       = 9
	LeaderboardID   = "tictactoe_global"
	StatsCollection = "tictactoe"
	StatsKey        = "stats"

	OpMove   = 1
	OpState  = 2
	OpError  = 3
	OpSystem = 4

	StatusWaiting   = "waiting"
	StatusActive    = "active"
	StatusFinished  = "finished"
	StatusAbandoned = "abandoned"
)

var WinLines = [][3]int{
	{0, 1, 2}, {3, 4, 5}, {6, 7, 8},
	{0, 3, 6}, {1, 4, 7}, {2, 5, 8},
	{0, 4, 8}, {2, 4, 6},
}

var ErrorCodes = struct {
	InvalidPayload string
	InvalidSender  string
	MatchNotActive string
	NotYourTurn    string
	InvalidPos     string
	CellOccupied   string
	PlayerNotIn    string
	InvalidMode    string
	RateLimitRPC   string
	RateLimitMove  string
}{
	InvalidPayload: "INVALID_PAYLOAD",
	InvalidSender:  "INVALID_SENDER",
	MatchNotActive: "MATCH_NOT_ACTIVE",
	NotYourTurn:    "NOT_YOUR_TURN",
	InvalidPos:     "INVALID_POSITION",
	CellOccupied:   "CELL_OCCUPIED",
	PlayerNotIn:    "PLAYER_NOT_IN_MATCH",
	InvalidMode:    "INVALID_MODE",
	RateLimitRPC:   "RATE_LIMIT_RPC",
	RateLimitMove:  "RATE_LIMIT_MOVE",
}

const (
	RateLimitWindowSec    = 10
	RateLimitCreateMatch  = 10
	RateLimitFindMatch    = 20
	RateLimitListMatches  = 30
	RateLimitMovePerTick  = 1
)
