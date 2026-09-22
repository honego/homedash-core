package music

// 音乐曲目
type Track struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Aliases     []string `json:"aliases"`
	Artists     []Artist `json:"artists"`
	Album       Album    `json:"album"`
	Artwork     string   `json:"artwork"`
	DurationMS  int64    `json:"duration_ms"`
	DiscNumber  int      `json:"disc_number,omitempty"`
	TrackNumber int      `json:"track_number,omitempty"`
}

// 音乐艺术家
type Artist struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

// 音乐专辑
type Album struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Aliases []string `json:"aliases"`
}

// 周排行曲目
type WeeklyTrack struct {
	Track     Track `json:"track"`
	Score     int   `json:"score"`
	PlayCount int   `json:"play_count"`
}
