package ui

import (
	"fmt"
	"strings"
	"time"

	"orb/internal/canvas"
	"orb/internal/feeds"
)

// rotateEvery is how long each page of headlines shows.
const rotateEvery = 15 * time.Second

// newsGrid: the categories fill the top half across the whole width, so
// each gets room; weather (now, next hours, forecast) and the reader share
// the bottom half (Stevie, 2026-10-23).
func newsGrid(w int) []row {
	hours := slot{nil, 0, []slot{{drawHourly, 45, nil}, {drawDaily, 55, nil}}}
	switch {
	case w >= 180:
		hours.weight = 34
		return []row{
			{50, []slot{{newsColumns(3), 1, nil}}},
			{50, []slot{{drawNow, 24, nil}, hours, {drawReader, 42, nil}}},
		}
	case w >= 100:
		hours.weight = 36
		return []row{
			{50, []slot{{newsColumns(3), 1, nil}}},
			{50, []slot{{drawNow, 30, nil}, hours, {drawReader, 34, nil}}},
		}
	default:
		out := []row{{24, []slot{{drawReader, 1, nil}}}, {16, []slot{{drawNow, 1, nil}}}, {12, []slot{{drawDaily, 1, nil}}}}
		for i := range 6 {
			out = append(out, row{10, []slot{{newsPanel(i), 1, nil}}})
		}
		return out
	}
}

// newsColumns lays the six categories out as cols x rows inside one slot.
func newsColumns(cols int) drawFn {
	return func(r canvas.Region, v *View, t *Theme) {
		rows := (6 + cols - 1) / cols
		ws := make([]int, cols)
		for i := range ws {
			ws[i] = 1
		}
		hs := make([]int, rows)
		for i := range hs {
			hs[i] = 1
		}
		y := 0
		for ri, hh := range split(r.H, 0, hs) {
			x := 0
			for ci, ww := range split(r.W, 1, ws) {
				newsPanel(ri*cols+ci)(r.Sub(x, y, ww, hh), v, t)
				x += ww + 1
			}
			y += hh
		}
	}
}

// accents cycles the theme's panel colors over the news categories.
func (t *Theme) accent(i int) canvas.Color {
	a := []canvas.Color{t.Temp, t.Net, t.CPU, t.Disk, t.Mem, t.GPU}
	return a[i%len(a)]
}

func newsPanel(i int) drawFn {
	return func(r canvas.Region, v *View, t *Theme) {
		f := v.Feeds
		if f.News == nil || i >= len(f.News.Categories) {
			name := "News"
			if f.News != nil {
				return // fewer categories than slots: leave the space empty
			}
			in := Frame(r, t, name, "", t.accent(i))
			if i == 0 {
				feedStatus(in, t, f.NewsOn, f.NewsErr, "news")
			}
			return
		}
		c := f.News.Categories[i]
		accent := t.accent(i)
		// One line per headline (the reader carries the long form), two when
		// the pane holds every headline that way: no cut-off titles.
		lines := 1
		if 2*len(c.Items) <= r.H-2 {
			lines = 2
		}
		per := max((r.H-2)/lines, 1)
		items := c.Items[:min(len(c.Items), 4*per)]
		pages := max((len(items)+per-1)/per, 1)
		page := int((v.Now.Unix()/int64(rotateEvery/time.Second))+int64(i)) % pages
		sub := ""
		if pages > 1 {
			sub = fmt.Sprintf("%d/%d", page+1, pages)
		}
		if f.NewsErr != "" {
			sub = f.NewsErr + "  " + sub
		}
		in := Frame(r, t, c.Name, sub, accent)
		if in.Empty() {
			return
		}
		y := 0
		for _, it := range items[page*per : min((page+1)*per, len(items))] {
			if y >= in.H {
				break
			}
			age, col := ageOf(it.Published, v.Now, t, accent)
			in.Bold(4-len(age), y, age, col)
			tw := in.W - 5
			wrapped := wrap(it.Title, tw, min(lines, in.H-y))
			for j, l := range wrapped {
				in.Text(5, y+j, l, t.FG)
			}
			// Feed name after the title when it fits.
			last := wrapped[len(wrapped)-1]
			if room := tw - len([]rune(last)) - 2; room >= 6 {
				in.TextRight(in.W, y+len(wrapped)-1, trunc(it.Feed, room), t.Faint)
			}
			y += lines
		}
	}
}

func feedStatus(in canvas.Region, t *Theme, on bool, errText, what string) {
	switch {
	case !on:
		in.Text(0, 0, "no "+what+" source configured", t.Dim)
		in.Text(0, 1, "set "+what+"_url in ~/.config/orb/config", t.Faint)
	case errText != "":
		in.Text(0, 0, what+": "+errText+", retrying", t.Temp)
	default:
		in.Text(0, 0, "fetching "+what+"…", t.Dim)
	}
}

// ageOf formats how old a headline is and picks a color: fresh is bright.
func ageOf(pub, now time.Time, t *Theme, accent canvas.Color) (string, canvas.Color) {
	if pub.IsZero() {
		return "--", t.Faint
	}
	d := now.Sub(pub)
	switch {
	case d < time.Minute:
		return "now", accent
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes())), accent
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours())), t.FG
	case d < 100*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24)), t.Dim
	default:
		return "old", t.Faint
	}
}

// wrap splits s into at most n lines of width w on word boundaries; the
// last line is truncated with an ellipsis if text remains.
func wrap(s string, w, n int) []string {
	if w <= 0 || n <= 0 {
		return []string{""}
	}
	words := strings.Fields(s)
	var out []string
	cur := ""
	for i, word := range words {
		next := word
		if cur != "" {
			next = cur + " " + word
		}
		if len([]rune(next)) <= w {
			cur = next
			continue
		}
		if len(out) == n-1 {
			rest := strings.Join(words[i:], " ")
			if cur != "" {
				rest = cur + " " + rest
			}
			return append(out, trunc(rest, w))
		}
		if cur != "" {
			out = append(out, cur)
		}
		cur = trunc(word, w)
	}
	return append(out, cur)
}

// tempHeat maps °F onto the heat ramp: 20°F cold, 105°F hot.
func tempHeat(f int) float64 { return max(0, min(1, (float64(f)-20)/85)) }

func drawNow(r canvas.Region, v *View, t *Theme) {
	w := v.Feeds.Weather
	title, sub := "Weather", ""
	if w != nil {
		title = w.Location
		if !w.Updated.IsZero() {
			sub = "updated " + w.Updated.Local().Format("3:04 PM")
		}
	}
	if v.Feeds.WeatherErr != "" {
		sub = v.Feeds.WeatherErr
	}
	in := Frame(r, t, title, sub, t.Host)
	if in.Empty() {
		return
	}
	if w == nil {
		feedStatus(in, t, v.Feeds.WeatherOn, v.Feeds.WeatherErr, "weather")
		return
	}
	n := w.Now
	x := 0
	sw, sh := spriteSize(in.Square())
	content := 7 // big temperature (3) + gap + summary, high/low, rain
	top := max((in.H-max(content, sh))/2, 0)
	if in.W >= sw+3+16 && in.H >= sh {
		drawSprite(in, 0, top+(max(content, sh)-sh)/2, n.Kind, v.Now, t)
		x = sw + 3
	}
	y0 := top + (max(content, sh)-content)/2
	temp := fmt.Sprint(n.Temp)
	tx := Big(in, x, y0, temp, func(int) canvas.Color { return t.Heat(tempHeat(n.Temp)) })
	in.Bold(tx+1, y0, "°F", t.Heat(tempHeat(n.Temp)))
	y := y0 + 4
	in.Bold(x, y, n.Summary, t.FG)
	hx := in.Text(x, y+1, "high ", t.Dim)
	hx = in.Bold(hx, y+1, fmt.Sprintf("%d°", n.High), t.Heat(tempHeat(n.High)))
	hx = in.Text(hx+2, y+1, "low ", t.Dim)
	in.Bold(hx, y+1, fmt.Sprintf("%d°", n.Low), t.Heat(tempHeat(n.Low)))
	if n.Precip > 0 {
		px := in.Text(x, y+2, "rain ", t.Dim)
		px = in.Bold(px, y+2, fmt.Sprintf("%d%%", n.Precip), t.RX)
		Gauge(in, px+1, y+2, min(12, in.W-px-1), float64(n.Precip)/100, t, func(float64) canvas.Color { return t.RX })
	}
}

func drawHourly(r canvas.Region, v *View, t *Theme) {
	in := Frame(r, t, "Next hours", "", t.CPU)
	w := v.Feeds.Weather
	if in.Empty() || w == nil || len(w.Hourly) == 0 {
		return
	}
	hrs := w.Hourly
	cw := in.W / len(hrs)
	step := 1
	for cw < 5 && step < len(hrs) {
		step++
		cw = in.W / ((len(hrs) + step - 1) / step)
	}
	lo, hi := hrs[0].Temp, hrs[0].Temp
	for _, h := range hrs {
		lo, hi = min(lo, h.Temp), max(hi, h.Temp)
	}
	// Temperature curve over the labels, two samples per cell column.
	if gh := in.H - 3; gh >= 2 {
		var vals []float64
		for i := 0; i < len(hrs); i += step {
			f := 0.15 + 0.85*float64(hrs[i].Temp-lo)/float64(max(hi-lo, 1))
			for range cw * 2 {
				vals = append(vals, f)
			}
		}
		Graph(in.Sub(0, 0, len(vals)/2, gh), vals, false, t, func(l float64) canvas.Color {
			return t.Heat(tempHeat(lo + int(l*float64(hi-lo))))
		})
	}
	for i, col := 0, 0; i < len(hrs); i, col = i+step, col+1 {
		h := hrs[i]
		x := col * cw
		mid := x + cw/2
		ts := fmt.Sprintf("%d°", h.Temp)
		in.Bold(mid-len([]rune(ts))/2, in.H-3, ts, t.Heat(tempHeat(h.Temp)))
		miniIcon(in, mid-1, in.H-2, h.Kind, t)
		label := hourLabel(h.Label, cw-1)
		in.Text(mid-len(label)/2, in.H-1, label, t.Dim)
	}
}

func drawDaily(r canvas.Region, v *View, t *Theme) {
	in := Frame(r, t, "Forecast", "", t.Disk)
	w := v.Feeds.Weather
	if in.Empty() || w == nil || len(w.Daily) == 0 {
		return
	}
	lo, hi := w.Daily[0].Low, w.Daily[0].High
	for _, d := range w.Daily {
		lo, hi = min(lo, d.Low), max(hi, d.High)
	}
	sumW := 0
	if in.W >= 60 {
		sumW = min(24, in.W-46)
	}
	barW := in.W - 4 - 3 - 5 - 5 - sumW - 1
	step := max(1, len(w.Daily)*2/max(in.H, 1))
	for i, y := 0, 0; i < len(w.Daily) && y < in.H; i, y = i+1, y+step {
		d := w.Daily[i]
		name := d.Name
		if len(name) > 3 {
			name = name[:3]
		}
		in.Bold(0, y, name, t.FG)
		miniIcon(in, 4, y, d.Kind, t)
		in.TextRight(12, y, fmt.Sprintf("%d°", d.Low), t.Heat(tempHeat(d.Low)))
		RangeBar(in, 13, y, barW, d.Low, d.High, lo, hi, t)
		in.Bold(13+barW+1, y, fmt.Sprintf("%d°", d.High), t.Heat(tempHeat(d.High)))
		if sumW > 0 {
			in.Text(in.W-sumW, y, trunc(d.Summary, sumW), t.Dim)
		}
	}
}

// RangeBar draws lo..hi as a lit segment on a track spanning min..max, each
// cell colored by the temperature at its position.
func RangeBar(r canvas.Region, x, y, w, lo, hi, minT, maxT int, t *Theme) {
	if w <= 0 {
		return
	}
	span := float64(max(maxT-minT, 1))
	a := int(float64(lo-minT) / span * float64(w))
	b := int(float64(hi-minT)/span*float64(w) + 0.999)
	for i := range w {
		r.SetBG(x+i, y, t.Track)
		if i >= a && i < max(b, a+1) {
			temp := minT + int(float64(i)/float64(w)*span)
			r.Set(x+i, y, '█', t.Heat(tempHeat(temp)))
		}
	}
}

// Weather art: '.' is transparent; S sun, s ray, C cloud, c cloud shade,
// R rain, L lightning, F snow, M mist. 12x8 pixels.
var sprites = map[feeds.Kind][8]string{
	feeds.Clear:        {"...s....s...", "....SSSS....", ".s.SSSSSS.s.", "..SSSSSSSS..", "..SSSSSSSS..", ".s.SSSSSS.s.", "....SSSS....", "...s....s..."},
	feeds.PartlyCloudy: {".s..s.......", "..SSS.......", "sSSS.CCC....", ".SSCCCCCCC..", "..CCCCCCCCC.", ".CCCCCCCCCCC", ".cccccccccc.", "............"},
	feeds.Cloudy:       {"............", "....CCC.....", "..CCCCCCC...", ".CCCCCCCCCC.", "CCCCCCCCCCCC", "cccccccccccc", ".cccccccccc.", "............"},
	feeds.Fog:          {"............", "MMMMMMMM....", "............", "..MMMMMMMMMM", "............", "MMMMMMMMMM..", "............", "...MMMMMMMMM"},
	feeds.Rain:         {"....CCC.....", "..CCCCCCC...", ".CCCCCCCCCC.", "cccccccccccc", ".cccccccccc.", "..R..R..R...", ".R..R..R....", "R..R..R....."},
	feeds.Storm:        {"....ccc.....", "..ccccccc...", ".cccccccccc.", "cccccLcccccc", ".cccLLccccc.", "..RLL..R..R.", ".R.LR.R..R..", "..L........."},
	feeds.Snow:         {"....CCC.....", "..CCCCCCC...", ".CCCCCCCCCC.", "cccccccccccc", ".cccccccccc.", "..F...F...F.", "....F...F...", "..F...F...F."},
}

func spritePalette(t *Theme) map[byte]canvas.Color {
	return map[byte]canvas.Color{
		'S': t.Heat(0.8), 's': t.Heat(0.62), 'C': t.FG, 'c': t.Dim,
		'R': t.RX, 'L': t.Heat(0.5), 'F': t.FG, 'M': t.Faint,
	}
}

// spriteSize is the sprite's size in cells: square tiles take one pixel per
// cell, terminal cells (twice as tall) take two pixels per cell.
func spriteSize(square bool) (w, h int) {
	if square {
		return 12, 8
	}
	return 12, 4
}

// drawSprite draws the weather art for k. Precipitation falls: its rows
// (5-7) cycle once per second.
func drawSprite(r canvas.Region, x, y int, k feeds.Kind, now time.Time, t *Theme) {
	art, ok := sprites[k]
	if !ok {
		art = sprites[feeds.Cloudy]
	}
	if k == feeds.Rain || k == feeds.Snow || k == feeds.Storm {
		shift := int(now.Unix() % 3)
		rows := [3]string{art[5], art[6], art[7]}
		for i := range 3 {
			art[5+i] = rows[(i-shift+3)%3]
		}
	}
	pal := spritePalette(t)
	if r.Square() {
		for py, line := range art {
			for px := range len(line) {
				if c, ok := pal[line[px]]; ok {
					r.Set(x+px, y+py, '█', c)
				}
			}
		}
		return
	}
	for cy := range 4 {
		top, bot := art[cy*2], art[cy*2+1]
		for px := range len(top) {
			ct, okT := pal[top[px]]
			cb, okB := pal[bot[px]]
			switch {
			case okT && okB:
				r.Set(x+px, y+cy, '▀', ct)
				r.SetBG(x+px, y+cy, cb)
			case okT:
				r.Set(x+px, y+cy, '▀', ct)
			case okB:
				r.Set(x+px, y+cy, '▄', cb)
			}
		}
	}
}

// mini weather icons: 2x2 pixels in two half-block cells.
var minis = map[feeds.Kind][2]string{
	feeds.Clear: {"SS", "SS"}, feeds.PartlyCloudy: {"SC", "CC"}, feeds.Cloudy: {"CC", "cc"},
	feeds.Fog: {"MM", "MM"}, feeds.Rain: {"CC", "RR"}, feeds.Storm: {"cc", "LR"}, feeds.Snow: {"CC", "FF"},
}

func miniIcon(r canvas.Region, x, y int, k feeds.Kind, t *Theme) {
	m, ok := minis[k]
	if !ok {
		m = minis[feeds.Cloudy]
	}
	pal := spritePalette(t)
	for i := range 2 {
		r.Set(x+i, y, '▀', pal[m[0][i]])
		r.SetBG(x+i, y, pal[m[1][i]])
	}
}

// hourLabel squeezes "11 AM" to "11AM", or to "11" when even that is wider
// than w.
func hourLabel(s string, w int) string {
	s = strings.ReplaceAll(s, " ", "")
	if len(s) <= w {
		return s
	}
	digits := strings.TrimRight(s, "AMPamp")
	return trunc(digits, max(w, 1))
}
