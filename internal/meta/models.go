package meta

type Meta struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Poster     string `json:"poster"`
	ImdbRating string `json:"imdbRating"`
	Year       string `json:"year"`
}

type CatalogResponse struct {
	Metas []Meta `json:"metas"`
}