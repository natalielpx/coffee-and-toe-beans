package cats

import (
	"fmt"
	"strings"
)

// Tier is how well a cat knows you. The bond behind it is never shown.
type Tier int

// The five tiers, from first meeting to "your cat".
const (
	Stranger Tier = iota
	Curious
	Familiar
	Friend
	YourCat
)

// TierNames are the tiers as written in content files.
var TierNames = []string{"stranger", "curious", "familiar", "friend", "your_cat"}

// tierFrom is the bond (0..100) where each tier begins.
var tierFrom = []float64{0, 10, 25, 45, 70}

// MaxBond is as close as a cat can get.
const MaxBond = 100

func (t Tier) String() string {
	if t < 0 || int(t) >= len(TierNames) {
		return fmt.Sprintf("tier(%d)", int(t))
	}
	return TierNames[t]
}

// TierOf turns a bond into its tier.
func TierOf(bond float64) Tier {
	t := Stranger
	for i, from := range tierFrom {
		if bond >= from {
			t = Tier(i)
		}
	}
	return t
}

// BondFor is the bond at the very start of a tier (handy for tests and
// previews).
func BondFor(t Tier) float64 {
	if t < 0 {
		t = 0
	}
	if int(t) >= len(tierFrom) {
		t = YourCat
	}
	return tierFrom[t]
}

// ParseTier reads a tier name from a content file ("" is stranger).
func ParseTier(s string) (Tier, error) {
	if s == "" {
		return Stranger, nil
	}
	for i, n := range TierNames {
		if strings.EqualFold(s, n) {
			return Tier(i), nil
		}
	}
	return 0, fmt.Errorf("unknown tier %q (tiers are %s)", s, strings.Join(TierNames, ", "))
}

// maxNear is how close to your seat a cat comes on its own at a tier: 0
// only the far spots, 1 nearby, 2 right onto your table. Shy cats keep
// their distance from strangers.
func maxNear(t Tier, shy bool) int {
	switch {
	case t >= Friend:
		return 2
	case t == Stranger && shy:
		return 0
	}
	return 1
}

// clampBond keeps a bond between 0 and MaxBond.
func clampBond(b float64) float64 {
	if b < 0 {
		return 0
	}
	if b > MaxBond {
		return MaxBond
	}
	return b
}
