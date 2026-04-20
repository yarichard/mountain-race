package meteo

import "testing"

// square ring: corners (lon, lat) — (0,0) (1,0) (1,1) (0,1) closed back to (0,0).
var unitSquare = [][]float64{
	{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0},
}

func TestPointInRing(t *testing.T) {
	tests := []struct {
		name string
		lat  float64
		lon  float64
		want bool
	}{
		{"centre", 0.5, 0.5, true},
		{"near edge inside", 0.1, 0.1, true},
		{"outside right", 0.5, 1.5, false},
		{"outside left", 0.5, -0.5, false},
		{"outside above", 1.5, 0.5, false},
		{"outside below", -0.5, 0.5, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pointInRing(unitSquare, tc.lat, tc.lon)
			if got != tc.want {
				t.Errorf("pointInRing(lat=%v, lon=%v) = %v, want %v", tc.lat, tc.lon, got, tc.want)
			}
		})
	}
}

func TestPointInMultiPolygon(t *testing.T) {
	// Two separate unit squares: one at (lon 0-1, lat 0-1) and one at (lon 2-3, lat 2-3).
	square1 := [][]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}
	square2 := [][]float64{{2, 2}, {3, 2}, {3, 3}, {2, 3}, {2, 2}}
	// A polygon with a hole: exterior (lon 0-4, lat 0-4), hole (lon 1-3, lat 1-3).
	outer := [][]float64{{0, 0}, {4, 0}, {4, 4}, {0, 4}, {0, 0}}
	hole := [][]float64{{1, 1}, {3, 1}, {3, 3}, {1, 3}, {1, 1}}

	multiSquares := [][][][]float64{
		{square1},
		{square2},
	}
	withHole := [][][][]float64{
		{outer, hole},
	}

	tests := []struct {
		name   string
		coords [][][][]float64
		lat    float64
		lon    float64
		want   bool
	}{
		{"in first polygon", multiSquares, 0.5, 0.5, true},
		{"in second polygon", multiSquares, 2.5, 2.5, true},
		{"between polygons", multiSquares, 1.5, 1.5, false},
		{"outside both", multiSquares, 5.0, 5.0, false},
		{"in outer but not hole", withHole, 0.5, 0.5, true},
		{"in hole — excluded", withHole, 2.0, 2.0, false},
		{"outside outer", withHole, 5.0, 5.0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := pointInMultiPolygon(tc.coords, tc.lat, tc.lon)
			if got != tc.want {
				t.Errorf("pointInMultiPolygon(lat=%v, lon=%v) = %v, want %v", tc.lat, tc.lon, got, tc.want)
			}
		})
	}
}
