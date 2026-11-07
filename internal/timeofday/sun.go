package timeofday

import (
	"math"
	"time"
)

// SunTimes works out sunrise and sunset for a date at a place, in minutes
// after local midnight (in the date's own time zone). It uses the standard
// NOAA approximation, accurate to a minute or two, which is plenty for
// lighting a café. ok is false where the sun doesn't rise or set that day.
func SunTimes(date time.Time, lat, lon float64) (rise, set float64, ok bool) {
	y, m, d := date.Date()
	noon := time.Date(y, m, d, 12, 0, 0, 0, date.Location())
	_, offset := noon.Zone()

	g := 2 * math.Pi / 365 * float64(noon.YearDay()-1)
	eqTime := 229.18 * (0.000075 + 0.001868*math.Cos(g) - 0.032077*math.Sin(g) -
		0.014615*math.Cos(2*g) - 0.040849*math.Sin(2*g))
	decl := 0.006918 - 0.399912*math.Cos(g) + 0.070257*math.Sin(g) -
		0.006758*math.Cos(2*g) + 0.000907*math.Sin(2*g) -
		0.002697*math.Cos(3*g) + 0.00148*math.Sin(3*g)

	latR := lat * math.Pi / 180
	cosH := math.Cos(90.833*math.Pi/180)/(math.Cos(latR)*math.Cos(decl)) - math.Tan(latR)*math.Tan(decl)
	if cosH < -1 || cosH > 1 {
		return 0, 0, false
	}
	h := math.Acos(cosH) * 180 / math.Pi
	local := float64(offset) / 60
	rise = 720 - 4*(lon+h) - eqTime + local
	set = 720 - 4*(lon-h) - eqTime + local
	return rise, set, true
}

// styledMinute moves a real clock time onto the café's "model day", whose
// sunrise and sunset are at fixed times. Real sunrise lands on the model
// sunrise, real sunset on the model sunset, and the hours between are
// stretched or squeezed evenly, so a December evening at 18:00 already looks
// like night, and a June evening at 21:00 still glows.
func styledMinute(m, rise, set, styledRise, styledSet float64) float64 {
	switch {
	case m < rise:
		return m / rise * styledRise
	case m < set:
		return styledRise + (m-rise)/(set-rise)*(styledSet-styledRise)
	default:
		return styledSet + (m-set)/(1440-set)*(1440-styledSet)
	}
}
