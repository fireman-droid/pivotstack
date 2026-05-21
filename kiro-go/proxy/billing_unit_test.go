package proxy

import (
	"math"
	"testing"
)

func TestQuotaToPivotDollars(t *testing.T) {
	tests := []struct {
		name                     string
		quota                    int64
		quotaPerUnitDollar       float64
		yuanPerUpstreamDollar    float64
		pivotStackDollarsPerYuan float64
		markup                   float64
		want                     float64
	}{
		{
			name:                     "apijing quota 29 markup 2",
			quota:                    29,
			quotaPerUnitDollar:       500000,
			yuanPerUpstreamDollar:    1,
			pivotStackDollarsPerYuan: 20,
			markup:                   2,
			want:                     0.00232,
		},
		{
			name:                     "hypothetical x one yuan equals ten upstream dollars",
			quota:                    29,
			quotaPerUnitDollar:       500000,
			yuanPerUpstreamDollar:    0.1,
			pivotStackDollarsPerYuan: 20,
			markup:                   2,
			want:                     0.000232,
		},
		{
			name:                     "markup one no profit",
			quota:                    29,
			quotaPerUnitDollar:       500000,
			yuanPerUpstreamDollar:    1,
			pivotStackDollarsPerYuan: 20,
			markup:                   1,
			want:                     0.00116,
		},
		{
			name:                     "zero quota per unit returns zero",
			quota:                    29,
			quotaPerUnitDollar:       0,
			yuanPerUpstreamDollar:    1,
			pivotStackDollarsPerYuan: 20,
			markup:                   2,
			want:                     0,
		},
		{
			name:                     "zero yuan multiplier returns zero",
			quota:                    29,
			quotaPerUnitDollar:       500000,
			yuanPerUpstreamDollar:    0,
			pivotStackDollarsPerYuan: 20,
			markup:                   2,
			want:                     0,
		},
		{
			name:                     "zero pivot unit returns zero",
			quota:                    29,
			quotaPerUnitDollar:       500000,
			yuanPerUpstreamDollar:    1,
			pivotStackDollarsPerYuan: 0,
			markup:                   2,
			want:                     0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := QuotaToPivotDollars(tt.quota, tt.quotaPerUnitDollar, tt.yuanPerUpstreamDollar, tt.pivotStackDollarsPerYuan, tt.markup)
			if math.Abs(got-tt.want) > 1e-12 {
				t.Fatalf("QuotaToPivotDollars() = %.12f, want %.12f", got, tt.want)
			}
		})
	}
}
