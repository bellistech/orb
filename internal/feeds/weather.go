package feeds

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Kind is a weather condition the UI can draw.
type Kind uint8

// Conditions, roughly in order of how much they matter.
const (
	Unknown Kind = iota
	Clear
	PartlyCloudy
	Cloudy
	Fog
	Rain
	Storm
	Snow
)

// Weather is a parsed, validated forecast.
type Weather struct {
	Location string
	Updated  time.Time
	Now      Now
	Hourly   []Hour
	Daily    []Day
}

// Now is current conditions, °F.
type Now struct {
	Temp, High, Low int
	Precip          int // chance, percent
	Summary         string
	Kind            Kind
}

// Hour is one hourly forecast entry.
type Hour struct {
	Label string
	Temp  int
	Kind  Kind
}

// Day is one daily forecast entry.
type Day struct {
	Name      string
	High, Low int
	Summary   string
	Kind      Kind
}

type rawWeather struct {
	Location string `json:"location"`
	Updated  string `json:"updated"`
	Now      *struct {
		Temp    *int   `json:"temp"`
		Summary string `json:"summary"`
		Icon    string `json:"icon"`
		High    int    `json:"high"`
		Low     int    `json:"low"`
		Precip  int    `json:"precip_chance"`
	} `json:"now"`
	Hourly []struct {
		Time string `json:"time"`
		Temp int    `json:"temp"`
		Icon string `json:"icon"`
	} `json:"hourly"`
	Daily []struct {
		Day     string `json:"day"`
		High    int    `json:"high"`
		Low     int    `json:"low"`
		Summary string `json:"summary"`
		Icon    string `json:"icon"`
	} `json:"daily"`
}

const (
	maxHours = 48
	maxDays  = 14
)

func validTemp(t int) bool { return t >= -80 && t <= 140 }

// ParseWeather decodes and validates the weather feed.
func ParseWeather(b []byte) (*Weather, error) {
	var r rawWeather
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("weather: %w", err)
	}
	if r.Now == nil || r.Now.Temp == nil {
		return nil, fmt.Errorf("weather: no current conditions")
	}
	n := r.Now
	if !validTemp(*n.Temp) || !validTemp(n.High) || !validTemp(n.Low) {
		return nil, fmt.Errorf("weather: temperature out of range")
	}
	if n.Precip < 0 || n.Precip > 100 {
		return nil, fmt.Errorf("weather: precipitation %d%% out of range", n.Precip)
	}
	w := &Weather{
		Location: Clean(r.Location, 40),
		Now: Now{Temp: *n.Temp, High: n.High, Low: n.Low, Precip: n.Precip,
			Summary: Clean(n.Summary, 40), Kind: kindOf(n.Icon, n.Summary)},
	}
	w.Updated, _ = time.Parse(time.RFC3339Nano, r.Updated)
	for i, h := range r.Hourly {
		if i >= maxHours {
			break
		}
		if !validTemp(h.Temp) {
			return nil, fmt.Errorf("weather: hourly temperature out of range")
		}
		w.Hourly = append(w.Hourly, Hour{Label: Clean(h.Time, 8), Temp: h.Temp, Kind: kindOf(h.Icon, "")})
	}
	for i, d := range r.Daily {
		if i >= maxDays {
			break
		}
		if !validTemp(d.High) || !validTemp(d.Low) {
			return nil, fmt.Errorf("weather: daily temperature out of range")
		}
		w.Daily = append(w.Daily, Day{Name: Clean(d.Day, 12), High: d.High, Low: d.Low,
			Summary: Clean(d.Summary, 40), Kind: kindOf(d.Icon, d.Summary)})
	}
	return w, nil
}

// kindOf reads the summary first (it is more specific than the icons the
// feed uses), then the emoji.
func kindOf(icon, summary string) Kind {
	s := strings.ToLower(summary)
	switch {
	case strings.Contains(s, "thunder") || strings.Contains(s, "storm"):
		return Storm
	case strings.Contains(s, "snow") || strings.Contains(s, "sleet") || strings.Contains(s, "flurr"):
		return Snow
	case strings.Contains(s, "rain") || strings.Contains(s, "shower") || strings.Contains(s, "drizzle"):
		return Rain
	case strings.Contains(s, "fog") || strings.Contains(s, "mist") || strings.Contains(s, "haze"):
		return Fog
	case strings.Contains(s, "partly") || strings.Contains(s, "mostly sunny"):
		return PartlyCloudy
	case strings.Contains(s, "cloud") || strings.Contains(s, "overcast"):
		return Cloudy
	case strings.Contains(s, "sunny") || strings.Contains(s, "clear"):
		return Clear
	}
	for _, r := range icon {
		switch r {
		case '☀':
			return Clear
		case '🌤', '⛅', '🌥':
			return PartlyCloudy
		case '☁':
			return Cloudy
		case '🌧', '🌦', '☔':
			return Rain
		case '⛈', '🌩':
			return Storm
		case '❄', '🌨':
			return Snow
		case '🌫':
			return Fog
		}
	}
	return Unknown
}
