package meta

type named struct {
	Name string `json:"name"`
}

type tmdbListItem struct {
	ID           int     `json:"id"`
	Title        string  `json:"title"`
	Name         string  `json:"name"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	ReleaseDate  string  `json:"release_date"`
	FirstAirDate string  `json:"first_air_date"`
	MediaType    string  `json:"media_type"`
	Character    string  `json:"character"`
	Popularity   float64 `json:"popularity"`
}

type tmdbCast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
}

type tmdbPerson struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Biography          string `json:"biography"`
	Birthday           string `json:"birthday"`
	Deathday           string `json:"deathday"`
	PlaceOfBirth       string `json:"place_of_birth"`
	KnownForDepartment string `json:"known_for_department"`
	ProfilePath        string `json:"profile_path"`
	ExternalIDs        struct {
		ImdbID string `json:"imdb_id"`
	} `json:"external_ids"`
	CombinedCredits struct {
		Cast []tmdbListItem `json:"cast"`
	} `json:"combined_credits"`
}

type tmdbPage struct {
	Results []tmdbListItem `json:"results"`
}

type tmdbCrew struct {
	Name       string `json:"name"`
	Job        string `json:"job"`
	Department string `json:"department"`
}

type tmdbVideo struct {
	Name string `json:"name"`
	Key  string `json:"key"`
	Site string `json:"site"`
	Type string `json:"type"`
}

type tmdbImage struct {
	FilePath string `json:"file_path"`
}

type tmdbDetail struct {
	ID                  int     `json:"id"`
	ImdbID              string  `json:"imdb_id"`
	Title               string  `json:"title"`
	Name                string  `json:"name"`
	Overview            string  `json:"overview"`
	PosterPath          string  `json:"poster_path"`
	BackdropPath        string  `json:"backdrop_path"`
	ReleaseDate         string  `json:"release_date"`
	FirstAirDate        string  `json:"first_air_date"`
	LastAirDate         string  `json:"last_air_date"`
	Runtime             int     `json:"runtime"`
	EpisodeRunTime      []int   `json:"episode_run_time"`
	Status              string  `json:"status"`
	Popularity          float64 `json:"popularity"`
	VoteAverage         float64 `json:"vote_average"`
	VoteCount           int     `json:"vote_count"`
	Genres              []named `json:"genres"`
	ProductionCountries []named `json:"production_countries"`
	CreatedBy           []named `json:"created_by"`
	Seasons             []struct {
		SeasonNumber int `json:"season_number"`
	} `json:"seasons"`
	LastEpisodeToAir *struct {
		Runtime int `json:"runtime"`
	} `json:"last_episode_to_air"`
	ExternalIDs struct {
		ImdbID string `json:"imdb_id"`
		TvdbID *int   `json:"tvdb_id"`
	} `json:"external_ids"`
	Credits struct {
		Cast []tmdbCast `json:"cast"`
		Crew []tmdbCrew `json:"crew"`
	} `json:"credits"`
	Videos struct {
		Results []tmdbVideo `json:"results"`
	} `json:"videos"`
	Images struct {
		Logos []tmdbImage `json:"logos"`
	} `json:"images"`
}

type tmdbEpisode struct {
	Name          string  `json:"name"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	AirDate       string  `json:"air_date"`
	StillPath     string  `json:"still_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
}
