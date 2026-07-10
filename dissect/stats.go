package dissect

// TradeWindowSeconds: a death is "traded" when the killer dies within this
// window of the kill. Deltas are computed on MatchUpdate.TimeElapsed (the
// monotonic round clock). Note the broadcast clock only has one-second
// resolution, so 3.5 effectively means "within 3 seconds, inclusive".
const TradeWindowSeconds = 3.5

type PlayerRoundStats struct {
	Username           string  `json:"username"`
	TeamIndex          int     `json:"-"`
	Score              int     `json:"score"`
	Operator           string  `json:"-"`
	Kills              int     `json:"kills"`
	Died               bool    `json:"died"`
	Assists            int     `json:"assists"`
	Headshots          int     `json:"headshots"`
	HeadshotPercentage float64 `json:"headshotPercentage"`
	OneVx              int     `json:"1vX,omitempty"`
	// Trades: kills this player made on an enemy within TradeWindowSeconds
	// of that enemy killing a teammate.
	Trades int `json:"trades"`
	// TradedDeaths: this player's deaths that a teammate avenged within
	// TradeWindowSeconds.
	TradedDeaths int `json:"tradedDeaths"`
}

type PlayerMatchStats struct {
	Username           string  `json:"username"`
	TeamIndex          int     `json:"-"`
	Rounds             int     `json:"rounds"`
	Kills              int     `json:"kills"`
	Deaths             int     `json:"deaths"`
	Assists            int     `json:"assists"`
	Headshots          int     `json:"headshots"`
	HeadshotPercentage float64 `json:"headshotPercentage"`
	Trades             int     `json:"trades"`
	TradedDeaths       int     `json:"tradedDeaths"`
}

// OpeningKill returns the first player to kill.
func (r *Reader) OpeningKill() MatchUpdate {
	for _, a := range r.MatchFeedback {
		if a.Type == Kill {
			return a
		}
	}
	return MatchUpdate{}
}

// OpeningDeath returns the first player to die (KILL or DEATH activity).
func (r *Reader) OpeningDeath() MatchUpdate {
	for _, a := range r.MatchFeedback {
		if a.Type == Kill || a.Type == Death {
			return a
		}
	}
	return MatchUpdate{}
}

// tradePairs returns every (original kill, avenging kill) pair: the second
// kill's victim is the first kill's killer, within TradeWindowSeconds on the
// monotonic clock. Not limited to adjacent feedback entries.
func (r *Reader) tradePairs() [][2]int {
	pairs := make([][2]int, 0)
	for i, a := range r.MatchFeedback {
		if a.Type != Kill {
			continue
		}
		for j := i + 1; j < len(r.MatchFeedback); j++ {
			b := r.MatchFeedback[j]
			if b.Type != Kill {
				continue
			}
			delta := b.TimeElapsed - a.TimeElapsed
			if delta > TradeWindowSeconds {
				break
			}
			if delta >= 0 && b.Target == a.Username {
				pairs = append(pairs, [2]int{i, j})
				break
			}
		}
	}
	return pairs
}

// markTrades stamps Traded on every Kill event. Runs once per round after
// the full feedback timeline is known.
func (r *Reader) markTrades() {
	traded := make(map[int]bool)
	for _, p := range r.tradePairs() {
		traded[p[0]] = true
	}
	for i := range r.MatchFeedback {
		if r.MatchFeedback[i].Type != Kill {
			continue
		}
		t := new(bool)
		*t = traded[i]
		r.MatchFeedback[i].Traded = t
	}
}

// Trades returns KILL Activity pairs of trades within TradeWindowSeconds.
func (r *Reader) Trades() [][]MatchUpdate {
	trades := make([][]MatchUpdate, 0)
	for _, p := range r.tradePairs() {
		trades = append(trades, []MatchUpdate{r.MatchFeedback[p[0]], r.MatchFeedback[p[1]]})
	}
	return trades
}

func (r *Reader) KillsAndDeaths() []MatchUpdate {
	MatchFeedback := make([]MatchUpdate, 0)
	for _, a := range r.MatchFeedback {
		if a.Type == Kill || a.Type == Death {
			MatchFeedback = append(MatchFeedback, a)
		}
	}
	return MatchFeedback
}

func (r *Reader) NumPlayers(team int) int {
	n := 0
	for _, p := range r.Header.Players {
		if p.TeamIndex == team {
			n++
		}
	}
	return n
}

func (r *Reader) PlayerStats() []PlayerRoundStats {
	stats := make([]PlayerRoundStats, 0)
	index := make(map[string]int)
	winningTeamIndex := 0
	if r.Header.Teams[1].Won {
		winningTeamIndex = 1
	}
	for i, p := range r.Header.Players {
		// The scoreboard can be shorter than the player list when a replay
		// is only partially parsed (e.g. unknown format) — don't panic.
		var scorePlayer ScoreboardPlayer
		if i < len(r.Scoreboard.Players) {
			scorePlayer = r.Scoreboard.Players[i]
		}
		stats = append(stats, PlayerRoundStats{
			Username:  p.Username,
			TeamIndex: p.TeamIndex,
			Operator:  p.Operator.String(),
			Assists:   int(scorePlayer.AssistsFromRound),
			Score:     int(scorePlayer.Score),
		})
		index[p.Username] = i
	}
	lastDeath := -1
	for _, a := range r.MatchFeedback {
		i := index[a.Username]
		if a.Type == Kill {
			stats[i].Kills += 1
			if *a.Headshot {
				stats[i].Headshots += 1
			}
			stats[i].HeadshotPercentage = headshotPercentage(stats[i].Headshots, stats[i].Kills)
			stats[index[a.Target]].Died = true
			lastDeath = index[a.Target]
		} else if a.Type == Death {
			stats[i].Died = true
			lastDeath = i
		}
	}
	// Trade attribution: the avenger gets a trade, the avenged victim gets
	// a traded death.
	for _, p := range r.tradePairs() {
		orig := r.MatchFeedback[p[0]]
		avenge := r.MatchFeedback[p[1]]
		if i, ok := index[avenge.Username]; ok {
			stats[i].Trades++
		}
		if i, ok := index[orig.Target]; ok {
			stats[i].TradedDeaths++
		}
	}
	// Calculates 1vX
	winnersLeftAlive := make([]int, 0)
	lastDeathWasWinner := false
	for i, p := range r.Header.Players {
		if p.TeamIndex != winningTeamIndex {
			continue
		}
		if !stats[i].Died {
			winnersLeftAlive = append(winnersLeftAlive, i)
		}
		if i == lastDeath {
			lastDeathWasWinner = true
		}
	}
	nWinnersLeftAlive := len(winnersLeftAlive)
	lastWinnerStanding := -1
	if nWinnersLeftAlive == 1 {
		lastWinnerStanding = winnersLeftAlive[0]
	} else if nWinnersLeftAlive == 0 && lastDeathWasWinner {
		lastWinnerStanding = lastDeath
	}
	if lastWinnerStanding > -1 {
		username := stats[lastWinnerStanding].Username
		teamLeft := r.NumPlayers(winningTeamIndex)
		oneVx := 0
		for _, a := range r.MatchFeedback {
			if a.Type == Kill && stats[index[a.Target]].TeamIndex == winningTeamIndex {
				teamLeft--
			} else if a.Type == Death && stats[index[a.Username]].TeamIndex == winningTeamIndex {
				teamLeft--
			} else if a.Type == PlayerLeave && stats[index[a.Username]].TeamIndex == winningTeamIndex {
				teamLeft--
			}
			if a.Username != username {
				continue
			}
			if a.Type == Kill && teamLeft < 2 {
				oneVx++
			}
		}
		for _, s := range stats {
			if s.TeamIndex != winningTeamIndex && !s.Died {
				oneVx++
			}
		}
		stats[lastWinnerStanding].OneVx = oneVx
	}
	return stats
}

func (m *MatchReader) PlayerStats() []PlayerMatchStats {
	stats := make([]PlayerMatchStats, 0)
	index := make(map[string]int)
	for i, r := range m.rounds {
		for _, p := range r.PlayerStats() {
			if len(stats) == 0 || stats[index[p.Username]].Username != p.Username {
				stats = append(stats, PlayerMatchStats{
					Username:  p.Username,
					TeamIndex: p.TeamIndex,
				})
				index[p.Username] = len(index)
			}
			i = index[p.Username]
			stats[i].Rounds += 1
			stats[i].Kills += p.Kills
			if p.Died {
				stats[i].Deaths += 1
			}
			stats[i].Assists += p.Assists
			stats[i].Headshots += p.Headshots
			stats[i].HeadshotPercentage = headshotPercentage(stats[i].Headshots, stats[i].Kills)
			stats[i].Trades += p.Trades
			stats[i].TradedDeaths += p.TradedDeaths
		}
	}
	return stats
}

func headshotPercentage(headshots, kills int) float64 {
	if kills == 0 {
		return 0
	}
	return float64(headshots) / float64(kills) * 100
}
