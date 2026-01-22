package tradeplus

//go:generate colgen --imports=tradebot/pkg/db
//colgen:Card,Review,Product
//colgen:Card:Index(NmID)
//colgen:Review:MapP(db),UniqueExternalID,Index(ExternalID)
//colgen:Product:MapP(db),Index(Article)

// MapP converts slice of type T to slice of type M with given converter with pointers.
func MapP[T, M any](a []T, f func(*T) *M) []M {
	n := make([]M, len(a))
	for i := range a {
		n[i] = *f(&a[i])
	}
	return n
}

func (ll Products) SetRecommendations(m map[int]Product) {
	for i, product := range ll {
		for _, rId := range product.RecommendationIDs {
			if v, ok := m[rId]; ok {
				ll[i].Recommendations = append(ll[i].Recommendations, v)
			}
		}
	}
}
