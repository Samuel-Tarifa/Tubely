package media

type jsonCmd struct {
	Streams []struct {
		Width  int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}
