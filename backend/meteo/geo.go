package meteo

// pointInMultiPolygon tests whether lat/lon lies inside any polygon of a
// GeoJSON MultiPolygon (coordinates are [lon, lat] per GeoJSON spec).
func pointInMultiPolygon(coords [][][][]float64, lat, lon float64) bool {
	for _, polygon := range coords {
		if len(polygon) == 0 {
			continue
		}
		if !pointInRing(polygon[0], lat, lon) {
			continue
		}
		inHole := false
		for _, hole := range polygon[1:] {
			if pointInRing(hole, lat, lon) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

// pointInRing implements ray-casting for a GeoJSON ring (each point is [lon, lat]).
func pointInRing(ring [][]float64, lat, lon float64) bool {
	inside := false
	n := len(ring)
	j := n - 1
	for i := 0; i < n; i++ {
		xi, yi := ring[i][0], ring[i][1] // lon, lat
		xj, yj := ring[j][0], ring[j][1]
		if ((yi > lat) != (yj > lat)) && (lon < (xj-xi)*(lat-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}
