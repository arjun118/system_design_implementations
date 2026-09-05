package suggest

type Reco struct {
	Word string
	Freq int64
}

// useful for seeding at Init

type Recos struct {
	Prefix string
	Top    []Reco
}
