package tradeplus

import "testing"

func TestReviewIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		r    Review
		want bool
	}{
		{name: "no content at all", r: Review{}, want: true},
		{name: "only WB metadata is still empty", r: Review{ReturnStatus: "available", ProductName: "насос", MatchingSize: "маломерит"}, want: true},
		{name: "has text", r: Review{Text: "отличный товар"}, want: false},
		{name: "has pros", r: Review{Pros: "быстрая доставка"}, want: false},
		{name: "has cons", r: Review{Cons: "помятая коробка"}, want: false},
		{name: "whitespace-only text is empty", r: Review{Text: "   \n\t"}, want: true},
		{name: "has bables", r: Review{Bables: []string{"размер"}}, want: false},
		{name: "has photos", r: Review{Photos: []string{"http://img"}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
