// Package schedule computes estimated route durations using Naismith's rule.
package schedule

// Naismith returns the estimated duration in hours.
//
// Rule: 1 hour per 5 km of distance + 1 hour per 600 m of ascent.
// Optional: add 10 min per 100 m of descent when descentM > 0.
func Naismith(distanceKm, ascentM, descentM float64) float64 {
	duration := (distanceKm / 5.0) + (ascentM / 600.0)
	if descentM > 0 {
		duration += (descentM / 1000.0) * (10.0 / 60.0)
	}
	return duration
}
