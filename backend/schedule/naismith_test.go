package schedule

import (
	"math"
	"testing"
)

func TestNaismith(t *testing.T) {
	tests := []struct {
		name       string
		distanceKm float64
		ascentM    float64
		descentM   float64
		wantMin    float64
		wantMax    float64
	}{
		{"flat 10km", 10, 0, 0, 1.9, 2.1},
		{"10km + 600m ascent", 10, 600, 0, 2.9, 3.1},
		{"with descent", 10, 600, 200, 2.9, 3.4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Naismith(tt.distanceKm, tt.ascentM, tt.descentM)
			if math.Abs(got-tt.wantMin) < -0.01 || got > tt.wantMax {
				t.Errorf("Naismith(%v, %v, %v) = %.2f, want [%.2f, %.2f]",
					tt.distanceKm, tt.ascentM, tt.descentM, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}
