package service

import "testing"

func TestVIPThresholdRefundAndDisabled(t *testing.T) {
	r := DefaultVIPRules()
	r.Enabled = true
	for _, c := range []struct {
		total float64
		level int
	}{{99.99, 0}, {100, 1}, {299.99, 1}, {300, 2}, {600, 3}, {1500, 4}, {3000, 5}, {90, 0}} {
		if got := r.Tier(c.total).Level; got != c.level {
			t.Fatalf("%v got %d want %d", c.total, got, c.level)
		}
	}
	r.Enabled = false
	if r.Tier(3000).Level != 0 {
		t.Fatal("disabled policy grants benefits")
	}
}
func TestVIPDecimalRatesAndPriceDecrease(t *testing.T) {
	for _, c := range []struct{ base, floor, cut, want float64 }{{.3, .2, .1, .225}, {.2, .17, .005, .195}, {.12, .11, .002, .118}, {.088, .088, 0, .088}, {.15, .2, .1, .15}} {
		if got := VIPDiscountedRate(c.base, c.floor, c.cut); got != c.want {
			t.Fatalf("got %.10f want %.10f", got, c.want)
		}
	}
}
func TestVIPRejectsUnsafePolicy(t *testing.T) {
	r := DefaultVIPRules()
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Groups = []VIPGroupRule{{GroupID: 1, Floor: .2, Discounts: []float64{.02, .04, .06, .08, .11}}}
	if r.Validate() == nil {
		t.Fatal("accepted discount above cap")
	}
	r.Groups[0].Discounts = []float64{0, 0, 0, 0, 0}
	r.Groups[0].Private = true
	r.Groups[0].Access = true
	if r.Validate() == nil {
		t.Fatal("accepted private automatic grant")
	}
}
