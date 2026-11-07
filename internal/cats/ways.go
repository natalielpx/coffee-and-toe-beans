package cats

import "fmt"

// Ways are a cat's habits, written as plain numbers in its content file so
// they can be tuned without touching code.
type Ways struct {
	// Shy cats never come close to a stranger on their own, and a sudden
	// choice makes them leave and never raises their bond.
	Shy bool `json:"shy"`
	// InRoom is roughly how much of the time the cat spends in the room,
	// from 0 (never) to 1 (always).
	InRoom float64 `json:"in_room"`
	// StayMinutes is how long the cat stays in one place before moving on
	// (shortest and longest, in café minutes).
	StayMinutes [2]float64 `json:"stay_minutes"`
	// AwayMinutes is how long the cat stays out of the room once it leaves.
	AwayMinutes [2]float64 `json:"away_minutes"`
	// Energy: above 1 the cat is livelier (shorter stays, fewer naps),
	// below 1 it's lazier.
	Energy float64 `json:"energy"`
	// Spots says how much the cat likes each place (bigger is more);
	// places left out are liked a little (1).
	Spots map[string]float64 `json:"spots"`
	// Drink is a drink the cat is curious about (for its moments).
	Drink string `json:"drink"`
}

// DefaultWays are used for anything a cat file leaves out.
func DefaultWays() Ways {
	return Ways{
		InRoom:      0.6,
		StayMinutes: [2]float64{8, 20},
		AwayMinutes: [2]float64{8, 20},
		Energy:      1,
	}
}

func (w Ways) check() error {
	if w.InRoom < 0 || w.InRoom > 1 {
		return fmt.Errorf("in_room should be between 0 and 1")
	}
	for _, r := range [][2]float64{w.StayMinutes, w.AwayMinutes} {
		if r[0] <= 0 || r[1] < r[0] {
			return fmt.Errorf("minutes should be [shortest, longest], both above 0")
		}
	}
	if w.Energy <= 0 {
		return fmt.Errorf("energy should be above 0 (1 is ordinary)")
	}
	for s, v := range w.Spots {
		if v < 0 {
			return fmt.Errorf("spots.%s should be 0 or more", s)
		}
	}
	return nil
}

// Likes is how much the cat likes a spot.
func (w Ways) Likes(spot string) float64 {
	if v, ok := w.Spots[spot]; ok {
		return v
	}
	return 1
}
