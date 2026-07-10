package dissect

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

func readTime(r *Reader) error {
	time, err := r.Uint32()
	if err != nil {
		return err
	}
	r.time = float64(time)
	// Track the action-phase length: the highest clock value seen. The prep
	// countdown (45s) and post-plant defuser timer (45s) never exceed it.
	if r.time > r.roundDuration {
		r.roundDuration = r.time
	}
	r.timeRaw = fmt.Sprintf("%d:%02d", time/60, time%60)
	return nil
}

func readY7Time(r *Reader) error {
	time, err := r.String()
	parts := strings.Split(time, ":")
	if len(parts) == 1 {
		seconds, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return err
		}
		r.time = seconds
		r.timeRaw = parts[0]
		return nil
	}
	minutes, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	seconds, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}
	r.time = float64((minutes * 60) + seconds)
	r.timeRaw = time
	return nil
}

func (r *Reader) roundEnd() {
	log.Debug().Msg("round_end")
	r.markTrades()

	planter := -1
	disabler := -1
	deaths := make(map[int]int)
	sizes := make(map[int]int)
	roles := make(map[int]TeamRole)

	for _, p := range r.Header.Players {
		sizes[p.TeamIndex] += 1
		roles[p.TeamIndex] = r.Header.Teams[p.TeamIndex].Role
	}

	for _, u := range r.MatchFeedback {
		switch u.Type {
		case Kill:
			i := r.Header.Players[r.PlayerIndexByUsername(u.Target)].TeamIndex
			deaths[i] = deaths[i] + 1
			// fix killer username
			if len(u.usernameFromScoreboard) > 0 {
				u.Username = u.usernameFromScoreboard
			}
		case Death:
			i := r.Header.Players[r.PlayerIndexByUsername(u.Username)].TeamIndex
			deaths[i] = deaths[i] + 1
		case DefuserPlantComplete:
			planter = r.PlayerIndexByUsername(u.Username)
		case DefuserDisableComplete:
			disabler = r.PlayerIndexByUsername(u.Username)
		}
	}

	// Y9S4+: StartingScore vs Score is the authoritative winner. Never let
	// objective events override it — a plant does not mean the planter's
	// team won (post-plant losses exist), and trusting it produced rounds
	// where BOTH teams had won=true. Objective events only pick the
	// win condition for the score-derived winner.
	if r.Header.CodeVersion >= Y9S4 {
		team0Won := r.Header.Teams[0].StartingScore < r.Header.Teams[0].Score
		r.Header.Teams[0].Won = team0Won
		r.Header.Teams[1].Won = !team0Won
		w := 1
		if team0Won {
			w = 0
		}
		loser := 1 - w
		switch {
		case disabler > -1 && r.Header.Players[disabler].TeamIndex == w:
			r.Header.Teams[w].WinCondition = DisabledDefuser
		case planter > -1 && r.Header.Players[planter].TeamIndex == w:
			if deaths[loser] == sizes[loser] {
				r.Header.Teams[w].WinCondition = KilledOpponents
			} else {
				r.Header.Teams[w].WinCondition = DefusedBomb
			}
		case deaths[loser] == sizes[loser]:
			r.Header.Teams[w].WinCondition = KilledOpponents
		default:
			r.Header.Teams[w].WinCondition = Time
		}
		return
	}

	// Legacy (<Y9S4): no score truth available — keep the original
	// event-driven inference.
	if disabler > -1 {
		i := r.Header.Players[disabler].TeamIndex
		r.Header.Teams[i].Won = true
		r.Header.Teams[i].WinCondition = DisabledDefuser
		return
	}

	if planter > -1 {
		r.Header.Teams[r.Header.Players[planter].TeamIndex].Won = true
		r.Header.Teams[r.Header.Players[planter].TeamIndex].WinCondition = DefusedBomb
		return
	}

	if deaths[0] == sizes[0] {
		r.Header.Teams[1].Won = true
		r.Header.Teams[1].WinCondition = KilledOpponents
		return
	}
	if deaths[1] == sizes[1] {
		r.Header.Teams[0].Won = true
		r.Header.Teams[0].WinCondition = KilledOpponents
		return
	}

	i := 0
	if roles[1] == Defense {
		i = 1
	}

	r.Header.Teams[i].Won = true
	r.Header.Teams[i].WinCondition = Time
}
