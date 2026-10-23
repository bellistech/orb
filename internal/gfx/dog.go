package gfx

import (
	"math"
	"unicode/utf8"

	"orb/internal/canvas"
	"orb/internal/ui"
)

// The dog: a small companion in the header whose behavior is a status
// signal (ui.DogMood). Hand-drawn 20x16 frames facing right, mirrored to
// face left; every pose is a pure function of the mood, its pen and the
// animation clock, so incremental and full renders agree.

const dogW, dogH = 20, 16

type dogFrame int

const (
	dogSit1 dogFrame = iota
	dogSit2
	dogPant1
	dogPant2
	dogBark
	dogNap1
	dogNap2
	dogTrot1
	dogTrot2
	dogSprint1
	dogSprint2
	dogBow1 // play bow, tail up
	dogBow2
	dogBat // a paw swipe
	dogLeap
	dogNose // head down to the ball
	dogFrames
)

// dogArt, drawn after a real dog, a small white terrier: '.' transparent,
// o outline and q the outline beside black fur (theme-tinted, q lighter so
// black ears stay apart from a dark header), w white coat, s and x its grey
// and dark ticking, k black (ears, eye patches, nose), h eye shine, t tan
// points (brows, cheeks, paws), c dark brown collar, g buckle, p tongue.
// The coat is the dog's own in every theme.
var dogArt = [dogFrames][dogH]string{
	dogSit1: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"......q.kkhkwwkhkq..",
		".......qktkwwwktwwq.",
		"........qtwwswwwwkkq",
		"........owwwwwwwppq.",
		".......occcccgccco..",
		"........owxwwwwswo..",
		"....oo..owwwsxwwwo..",
		"...owo.owsxwwwwswo..",
		"...oso.owwwwswwtwo..",
		"....oowwxwwswowtto..",
		".....owswwttowotto..",
		"......ooooooooooooo.",
	},
	dogSit2: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"......q.kkhkwwkhkq..",
		".......qktkwwwktwwq.",
		"........qtwwswwwwkkq",
		"........owwwwwwwppq.",
		".......occcccgccco..",
		"........owxwwwwswo..",
		"........owwwswwwwo..",
		"..oooo.owsswwwwswo..",
		".owwsoooowwwswwtwo..",
		"..ooooowswwwwowtto..",
		".....owswwttowotto..",
		"......ooooooooooooo.",
	},
	dogPant1: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"......q.kkhkwwkhkq..",
		".......qktkwwwktwwq.",
		"........qtwwswwwwkkq",
		"........owwwwwwoppq.",
		".......occcccgcppo..",
		"........owxwwwwpoo..",
		"....oo..owwwsxwwwo..",
		"...owo.owsxwwwwswo..",
		"...oso.owwwwswwtwo..",
		"....oowwxwwswowtto..",
		".....owswwttowotto..",
		"......ooooooooooooo.",
	},
	dogPant2: {
		"....................",
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"......q.kkhkwwkhkq..",
		".......qktkwwwktwwq.",
		"........qtwwswwwwkkq",
		".......oowwwwwwoppq.",
		".......occcccgcppo..",
		"....oo..owwwsxwpoo..",
		"...owo.owsxwwwwswo..",
		"...oso.owwwwswwtwo..",
		"....oowwxwwswowtto..",
		".....owswwttowotto..",
		"......ooooooooooooo.",
	},
	dogBark: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"......q.kkhkwwkhkq..",
		".......qktkwwwktwwq.",
		"........qtwwswwwwkkq",
		"........owwwwwoppq..",
		".......occcccgowwo..",
		"........owxwwwwooo..",
		"....oo..owwwsxwwwo..",
		"...owo.owsxwwwwswo..",
		"...oso.owwwwswwtwo..",
		"....oowwxwwswowtto..",
		".....owswwttowotto..",
		"......ooooooooooooo.",
	},
	dogNap1: {
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		"....oqkqkktkwwktkqkq",
		"..oowxqwkkkkwwkkkq..",
		".owswwwqktkwwwktwwq.",
		"owwxwwswqtwwswwwwkkq",
		"owswwxwwwowwwwwwoq..",
		".owwttwswwwwwttwtto.",
		"..ooooooooooooooooo.",
	},
	dogNap2: {
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		"...ooqkqkktkwwktkqkq",
		"..owxwqwkkkkwwkkkq..",
		".owswwwqktkwwwktwwq.",
		"owwxwwswqtwwswwwwkkq",
		"owswwxwwwowwwwwwoq..",
		".owwttwswwwwwttwtto.",
		"..ooooooooooooooooo.",
	},
	dogTrot1: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		".o....q.kkhkwwkhkq..",
		"owo....qktkwwwktwwq.",
		"owo.....qtwwswwwwkkq",
		".owo....owwwwwwwppq.",
		"..owwwswocccccgcco..",
		"..owxwwwwswwxwwswo..",
		"..owwswwxwwwwswwwo..",
		"..oswwxwwwwswwxwwo..",
		"...owwswwwxwwwswo...",
		"..osowo.....osowo...",
		"..otoot.....otoot...",
		"...oooo......oooo...",
	},
	dogTrot2: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		".o....q.kkhkwwkhkq..",
		"owo....qktkwwwktwwq.",
		"owo.....qtwwswwwwkkq",
		".owo....owwwwwwwppq.",
		"..owwwswocccccgcco..",
		"..owxwwwwswwxwwswo..",
		"..owwswwxwwwwswwwo..",
		"..oswwxwwwwswwxwwo..",
		"...owwswwwxwwwswo...",
		"....owo.......owo...",
		"....oto.......oto...",
		"....oo........oo....",
	},
	dogSprint1: {
		"....................",
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"o.....q.kkhkwwkhkq..",
		"ooo....qktkwwwktwwq.",
		".owoo...qtwwswwwwkkq",
		"..owwoo.owwwwwwwppq.",
		"..owswwwocccccgccco.",
		"..owwxwwswwwxswwwo..",
		"..oswwwswwwwwwswwo..",
		"..owwswwwwxwwwwwo...",
		".otoowo......owotto.",
		"otoo.........oottoo.",
		".o.............oo...",
	},
	dogSprint2: {
		"....................",
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"o.....q.kkhkwwkhkq..",
		"ooo....qktkwwwktwwq.",
		".owoo...qtwwswwwwkkq",
		"..owwoo.owwwwwwwppq.",
		"..owswwwocccccgccco.",
		"..owwxwwswwwxswwwo..",
		"..oswwwswwwwwwswwo..",
		"..owwswwwwxwwwwwo...",
		".....owo.oxwo.......",
		"......otootto.......",
		".......oo..oo.......",
	},
	dogBow1: {
		"....................",
		".o..................",
		"owo.................",
		"owo.................",
		".owoo...............",
		"..owwooqqqq....oqq..",
		"..owswqkkkkqoooqkkkq",
		"..owwqkkkkkswwskkkkq",
		"..oswqkqkktkwwktkqkq",
		"..owwsqwkkhkwwkhkq..",
		"..owxwwqktkwwwktwwq.",
		"..owwwswqtwwswwwwkkq",
		"..otwowsowwwwwwwppq.",
		"..otowwocccgccowwwwo",
		"..ottoxwwwwwtttttto.",
		"...ooooooooooooooo..",
	},
	dogBow2: {
		"....................",
		"....o...............",
		"...owo..............",
		"..owo...............",
		".owwoo..............",
		"..owwooqqqq....oqq..",
		"..owswqkkkkqoooqkkkq",
		"..owwqkkkkkswwskkkkq",
		"..oswqkqkktkwwktkqkq",
		"..owwsqwkkhkwwkhkq..",
		"..owxwwqktkwwwktwwq.",
		"..owwwswqtwwswwwwkkq",
		"..otwowsowwwwwwwppq.",
		"..otowwocccgccowwwwo",
		"..ottoxwwwwwtttttto.",
		"...ooooooooooooooo..",
	},
	dogBat: {
		"....................",
		".o..................",
		"owo.................",
		"owo.................",
		".owoo...............",
		"..owwooqqqq....oqq..",
		"..owswqkkkkqoooqkkkq",
		"..owwqkkkkkswwskkkkq",
		"..oswqkqkktkwwktkqkq",
		"..owwsqwkkhkwwkhkq..",
		"..owxwwqktkwwwktwwo.",
		"..owwwswqtwwswwwwooo",
		"..otwowsowwwwwwwptto",
		"..otowwocccgccowwtto",
		"..ottoxwwwwwtttotoo.",
		"...ooooooooooooooo..",
	},
	dogLeap: {
		".......qqqq....oqq..",
		"......qkkkkqoooqkkkq",
		".....qkkkkkswwskkkkq",
		".....qkqkktkwwktkqkq",
		"o.....q.kkhkwwkhkq..",
		"oo.....qktkwwwktwwq.",
		".owoo...qtwwswwwwkkq",
		"..owwoo.owwwwwwwppq.",
		"..owswwwocccccgccco.",
		"..owwxwwswwwxswwwo..",
		".ooswwwswwwwwwswwoo.",
		"ottowswwwwxwwwwwotto",
		"otoo.oooooooooo..oto",
		".o................o.",
		"....................",
		"....................",
	},
	dogNose: {
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		".o..................",
		"owo.................",
		".owoooo.............",
		"..owwxwqqqq....oqq..",
		"..owswqkkkkqoooqkkkq",
		"..oswqkkkkkswwskkkkq",
		"..owwqkqkktkwwktkqkq",
		"..osowqwkkhkwwkhkq..",
		"..otoooqktkwwwktwwq.",
		"..otoootqtwwswwwwkkq",
		"...oo..ooowwwwwwoq..",
	},
}

// The ball: orange with a blue stripe, 7x7, in four turns of the stripe
// (counterclockwise: a ball rolling left steps forward through them).
// r orange, R its shade, b blue, y highlight, o outline.
const ballW = 7

var ballArt = [4][ballW]string{
	{
		"..ooo..",
		".orrro.",
		"oryrrro",
		"obbbbbo",
		"orrrrRo",
		".orRRo.",
		"..ooo..",
	},
	{
		"..ooo..",
		".oyrbo.",
		"orrbbbo",
		"orbbbro",
		"obbbrRo",
		".obrRo.",
		"..ooo..",
	},
	{
		"..ooo..",
		".orbro.",
		"orybrro",
		"orrbrro",
		"orrbrRo",
		".orbRo.",
		"..ooo..",
	},
	{
		"..ooo..",
		".obrro.",
		"obbbyro",
		"orbbbro",
		"orrbbbo",
		".orRbo.",
		"..ooo..",
	},
}

// dogPalette maps art bytes to pixels; ok marks the opaque ones.
type dogPalette struct {
	v  [128]uint32
	ok [128]bool
}

func newDogPalette(t *ui.Theme, accent canvas.Color) *dogPalette {
	p := &dogPalette{}
	set := func(b byte, c canvas.Color) { p.v[b], p.ok[b] = rgb(c), true }
	set('o', ui.Mix(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.5), accent, 0.2))
	set('q', ui.Mix(t.Faint, accent, 0.35))
	set('w', canvas.RGB(0xf2, 0xef, 0xe8))
	set('s', canvas.RGB(0x9a, 0xa0, 0xae))
	set('x', canvas.RGB(0x55, 0x58, 0x66))
	set('k', canvas.RGB(0x22, 0x20, 0x26))
	set('t', canvas.RGB(0xc9, 0x8a, 0x4b))
	set('h', canvas.RGB(0xff, 0xff, 0xff))
	set('c', canvas.RGB(0x5a, 0x3a, 0x26))
	set('g', canvas.RGB(0xb8, 0xb4, 0xac))
	set('p', canvas.RGB(0xff, 0x6e, 0x8c))
	set('r', canvas.RGB(0xff, 0x7a, 0x1a))
	set('R', canvas.RGB(0xc8, 0x56, 0x0e))
	set('b', canvas.RGB(0x2a, 0x6f, 0xdb))
	set('y', canvas.RGB(0xff, 0xd2, 0xa8))
	return p
}

// dogPose is where the dog is, which frame it shows and where its ball is.
type dogPose struct {
	frame dogFrame
	x, y  int  // top-left of the sprite
	left  bool // facing left (mirrored)
	ball  ballPose
}

// ballPose places the ball: the top-left of its sprite, how far its stripe
// has turned, whether the dog holds it (drawn over the dog), and whether it
// is put away. It is out while the dog stays home or plays; running laps
// it is put away (a ball left at home across the pen from the dog would
// cost a second blit run on every header row), and barking the speech
// bubble is where it lies.
type ballPose struct {
	x, y int
	rot  int
	held bool
	hide bool
}

// Speeds in logical pixels per second.
const (
	dogTrotSpeed   = 22.0
	dogSprintSpeed = 75.0
	dogCarrySpeed  = 26.0 // bringing the ball home
)

// A bout of ball play, in seconds into it (ui.DogAct): a play bow, a swipe
// that sends the ball rolling away from home, a dash and a pounce on it,
// then the dog picks it up, trots it home, turns and drops it at its feet.
// The trip home takes up to 2 s, so a bout ends sitting with the ball home.
const (
	playBat   = 1.0 // bowing until here
	playHit   = 1.1 // the paw meets the ball
	playChase = 1.3
	playLeap  = 2.3
	playLand  = 2.7
	playNose  = 3.5
	playPick  = 3.7 // in the mouth
	playCarry = 3.9
	playTurn  = 0.4 // home: turn to face out,
	playDrop  = 0.4 // then let go
	playRollT = 1.4 // the ball rolls this long
	playRoll  = 44  // and at most this far, in pixels
)

// heldAt is where a held ball's sprite goes in right-facing frames: in the
// mouth, or on the ground at the nose.
var heldAt = [dogFrames][2]int{
	dogSit1:  {14, 6},
	dogSit2:  {14, 6},
	dogTrot1: {14, 6},
	dogTrot2: {14, 6},
	dogNose:  {12, 9},
}

// lerpPx moves from pixel a to b by v in [0, 1].
func lerpPx(a, b int, v float64) int { return a + int(math.Round(float64(b-a)*v)) }

// dogPoseAt places the dog and its ball in pen pr at time T. The dog
// stands on the pen's bottom edge, one pixel up so its shadow fits.
// Resting moods sit at the right end facing into the screen, the ball at
// their feet; trotting and sprinting run laps; play takes the ball on a
// trip and brings it back.
func dogPoseAt(m ui.Mood, pr rect, T float64) dogPose {
	lo, hi := pr.x0+2, pr.x1-dogW-2
	p := dogPose{x: hi, y: pr.y1 - 1 - dogH, left: true}
	home := max(hi-ballW-1, pr.x0)
	p.ball = ballPose{x: home, y: pr.y1 - 1 - ballW}
	step := func(period float64, n int) int { return int(math.Floor(T/period)) % n }
	lap := func(speed float64) {
		span := float64(hi - lo)
		if span <= 0 {
			return
		}
		d := math.Mod(T*speed, 2*span)
		if d < span {
			p.x, p.left = lo+int(d), false
		} else {
			p.x, p.left = hi-int(d-span), true
		}
	}
	m, u := ui.DogAct(m, T)
	switch m {
	case ui.Nap:
		p.frame = dogNap1 + dogFrame(step(1.1, 2)) // breathing
	case ui.Sit:
		p.frame = dogSit1 + dogFrame(step(0.22, 2)) // tail wag
		// Looks around: every 4 s, a 1.3 s glance the other way.
		p.left = math.Mod(T, 4) < 2.7
	case ui.Trot:
		p.frame = dogTrot1 + dogFrame(step(0.16, 2))
		lap(dogTrotSpeed)
		p.ball.hide = true
	case ui.Sprint:
		p.frame = dogSprint1 + dogFrame(step(0.08, 2))
		lap(dogSprintSpeed)
		p.ball.hide = true
	case ui.Pant:
		p.frame = dogPant1 + dogFrame(step(0.16, 2))
	case ui.Bark:
		// "Woof woof", then a breath: open mouth twice per 1.2 s.
		u := math.Mod(T, 1.2)
		p.frame = dogSit1
		if u < 0.18 || (u >= 0.32 && u < 0.5) {
			p.frame = dogBark
		}
		p.ball.hide = true
	case ui.Play:
		p.play(u, lo, hi, home, dogFrame(step(0.15, 2)))
	}
	// Airborne frames of a gait lift a pixel.
	if p.frame == dogTrot2 || p.frame == dogSprint2 {
		p.y--
	}
	if p.ball.held {
		o := heldAt[p.frame]
		p.ball.x, p.ball.y = p.x+o[0], p.y+o[1]
		if p.left {
			p.ball.x = p.x + dogW - o[0] - ballW
		}
	}
	return p
}

// play poses the dog and the ball u seconds into a bout of ball play; tail
// alternates 0 and 1 for the wag.
func (p *dogPose) play(u float64, lo, hi, home int, tail dogFrame) {
	roll := max(0, min(playRoll, home-lo-5))
	land := min(hi, home-roll+5)
	launch := min(hi, land+12)
	carried := playCarry + float64(hi-land)/dogCarrySpeed
	dropped := carried + playTurn
	switch {
	case u < playBat:
		p.frame = dogBow1 + tail
	case u < playChase:
		p.frame = dogBat
	case u < playLeap:
		p.frame = dogSprint1 + dogFrame(int(math.Floor(u/0.08))%2)
		p.x = lerpPx(hi, launch, (u-playChase)/(playLeap-playChase))
	case u < playLand:
		v := (u - playLeap) / (playLand - playLeap)
		p.frame = dogLeap
		p.x = lerpPx(launch, land, v)
		p.y -= int(math.Round(3 * math.Sin(math.Pi*v)))
	case u < playNose:
		p.frame, p.x = dogBow1+tail, land
	case u < playCarry:
		p.frame, p.x = dogNose, land
	case u < carried:
		p.frame = dogTrot1 + dogFrame(int(math.Floor(u/0.16))%2)
		p.x, p.left = lerpPx(land, hi, (u-playCarry)/(carried-playCarry)), false
	case u < dropped:
		p.frame = dogSit1
	default:
		p.frame = dogSit1 + tail
	}
	b := &p.ball
	switch {
	case u < playHit:
	case u < playPick:
		// Swiped: rolls left, slowing to a stop; the stripe turns with it.
		v := min((u-playHit)/playRollT, 1)
		d := lerpPx(0, roll, 1-(1-v)*(1-v))
		b.x, b.rot = home-d, d/2%4
	case u < dropped:
		b.held = true
	case u < dropped+playDrop:
		// Let go from the mouth: falls to the dog's feet with a bounce.
		v := (u - dropped) / playDrop
		o := heldAt[dogSit1]
		x0, y0 := p.x+dogW-o[0]-ballW, p.y+o[1]
		fall := min(v/0.6, 1)
		b.x = lerpPx(x0, home, v)
		b.y = lerpPx(y0, b.y, fall*fall)
		if v > 0.6 {
			b.y -= int(math.Round(3 * math.Sin(math.Pi*(v-0.6)/0.4)))
		}
		b.rot = int(v*4) % 4
	}
}

// dogMouth is where the mouth is in a right-facing frame (for barks).
const dogMouthX, dogMouthY = 18, 7

// dog draws the companion and its effects (shadow, nap z's, sprint dust,
// bark lines and the speech bubble), and its ball: two overlays, the ball
// first unless the dog holds it.
func (r *Renderer) dog(pt canvas.Pet, t *ui.Theme, a Anim) []overlay {
	pr := rect{pt.X * Tile, pt.Y * Tile, (pt.X + pt.W) * Tile, (pt.Y + pt.H) * Tile}
	m, _ := ui.DogAct(ui.Mood(pt.Mood), a.T)
	p := dogPoseAt(ui.Mood(pt.Mood), pr, a.T)
	if r.dogPal == nil || r.dogPal.theme != t || r.dogPal.accent != pt.Accent {
		r.dogPal = &dogPalCache{t, pt.Accent, newDogPalette(t, pt.Accent)}
	}
	pal := r.dogPal.p
	ground := pr.y1 - 1

	// Everything drawn stays inside bb: the sprite with room for its
	// effects, widened for the bubble.
	bb := rect{p.x - 8, pr.y0, p.x + dogW + 8, pr.y1}
	var bubble rect
	var lines [2]string
	if m == ui.Bark {
		lines = [2]string{"Woof!", pt.Say}
		bw := max(7*utf8.RuneCountInString(lines[0]), 6*utf8.RuneCountInString(lines[1])) + 7
		bubble = rect{p.x - 7 - bw, pr.y0 + 2, p.x - 7, ground}
		if !p.left { // mouth on the right: bubble there
			bubble = rect{p.x + dogW + 7, pr.y0 + 2, p.x + dogW + 7 + bw, ground}
		}
		bb = rect{min(bb.x0, bubble.x0-1), bb.y0, max(bb.x1, bubble.x1+4), bb.y1}
	}
	bb = rect{max(bb.x0, pr.x0-Tile), bb.y0, min(bb.x1, pr.x1+Tile), bb.y1}

	dog := overlay{bb, func(f *Frame) {
		blend := func(x, y int, v uint32, k float64) {
			if x >= bb.x0 && x < bb.x1 && y >= bb.y0 && y < bb.y1 && x >= 0 && y >= 0 && x < f.W && y < f.H {
				i := y*f.W + x
				f.Pix[i] = lerpRGB(f.Pix[i], v, k)
			}
		}
		put := func(x, y int, v uint32) { blend(x, y, v, 1) }

		// Shadow: a soft line under the paws.
		x0, x1 := p.x+2, p.x+dogW-1
		if m == ui.Nap {
			x0 = p.x
		}
		for x := x0; x < x1; x++ {
			k := 0.45
			if x == x0 || x == x1-1 {
				k = 0.2
			}
			blend(x, ground, 0, k)
		}

		drawArt(dogArt[p.frame][:], p.x, p.y, p.left, pal, put)

		fg := rgb(t.FG)
		switch m {
		case ui.Nap:
			// Three z's drift up from the head, staggered.
			hx := p.x + 13
			if p.left {
				hx = p.x + 3
			}
			for k := range 3 {
				const life = 2.4
				u := math.Mod(a.T+float64(k)*life/3, life) / life
				fade := min(u/0.15, 1, (1-u)/0.3)
				zx := hx + int(u*6)
				if p.left {
					zx = hx - int(u*6) - 3
				}
				zy := p.y + 6 - int(u*12)
				zz := [4]string{"####", "..#.", ".#..", "####"}
				if k%2 == 1 {
					zz = [4]string{"###.", ".#..", "###.", "...."}
				}
				for y, row := range zz {
					for x := range len(row) {
						if row[x] == '#' {
							blend(zx+x, zy+y, fg, fade)
						}
					}
				}
			}
		case ui.Sprint:
			// Dust kicked up behind.
			for k := range 3 {
				u := math.Mod(a.T*3+float64(k)/3, 1)
				dx := 2 + int(u*7)
				x := p.x - dx
				if p.left {
					x = p.x + dogW - 1 + dx
				}
				y := ground - 1 - int(u*3)
				c := rgb(ui.Mix(t.Dim, t.FG, 0.3))
				blend(x, y, c, 0.7*(1-u))
				blend(x+1, y, c, 0.5*(1-u))
				blend(x, y-1, c, 0.4*(1-u))
			}
		case ui.Pant:
			// Sweat drops flicking off the back of the head.
			drop := rgb(canvas.RGB(0x9b, 0xdc, 0xff))
			for k := range 2 {
				u := math.Mod(a.T/0.9+float64(k)*0.5, 1)
				dx := int(u * 7)
				hx := p.x + 6 - dx // behind the head (art faces right)
				if p.left {
					hx = p.x + dogW - 7 + dx
				}
				hy := p.y + 1 - int(8*u*(1-u)*2.2) + int(u*4)
				fade := min(1, (1-u)/0.35)
				blend(hx, hy, drop, fade)
				blend(hx, hy+1, drop, fade)
				blend(hx+1, hy+1, drop, fade)
				blend(hx-1, hy+1, drop, 0.5*fade)
			}
		case ui.Bark:
			hot := rgb(t.Heat(1))
			if p.frame == dogBark {
				// Sound lines fanning out from the mouth.
				mx, dir := p.x+dogMouthX+2, 1
				if p.left {
					mx, dir = p.x+dogW-1-dogMouthX-2, -1
				}
				for _, d := range [3][2]int{{1, -2}, {2, 0}, {1, 2}} {
					for s := range 2 {
						put(mx+dir*(d[0]+s), p.y+dogMouthY+d[1]*(1+s)/2, hot)
					}
				}
			}
			drawBubble(f, bubble, !p.left, lines, t, put)
		}
	}}
	if p.ball.hide {
		return []overlay{dog}
	}
	b := p.ball
	br := rect{b.x, b.y, b.x + ballW, b.y + ballW}
	if !b.held {
		br.y1 = max(br.y1, ground+1)
	}
	ball := overlay{br, func(f *Frame) {
		put := func(x, y int, v uint32) {
			if x >= br.x0 && x < br.x1 && y >= br.y0 && y < br.y1 && x >= 0 && y >= 0 && x < f.W && y < f.H {
				f.Pix[y*f.W+x] = v
			}
		}
		if !b.held {
			// Its shadow, fainter the higher it bounces.
			k := 0.4 * ballW / float64(ballW+ground-(b.y+ballW))
			for x := b.x + 1; x < b.x+ballW-1; x++ {
				if x >= 0 && ground >= 0 && ground < f.H && x < f.W {
					i := ground*f.W + x
					f.Pix[i] = lerpRGB(f.Pix[i], 0, k)
				}
			}
		}
		drawArt(ballArt[b.rot][:], b.x, b.y, false, pal, put)
	}}
	if b.held {
		return []overlay{dog, ball}
	}
	return []overlay{ball, dog}
}

// drawArt puts the opaque bytes of art at x, y, mirrored when flip.
func drawArt(art []string, x, y int, flip bool, pal *dogPalette, put func(x, y int, v uint32)) {
	for sy, row := range art {
		n := len(row)
		for sx := range n {
			b := row[sx]
			if flip {
				b = row[n-1-sx]
			}
			if b < 128 && pal.ok[b] {
				put(x+sx, y+sy, pal.v[b])
			}
		}
	}
}

type dogPalCache struct {
	theme  *ui.Theme
	accent canvas.Color
	p      *dogPalette
}

// drawBubble draws an RPG speech bubble: cream fill, dark rim, a tail
// toward the dog, the first line in the hot color and the rest dark.
func drawBubble(f *Frame, b rect, tailLeft bool, lines [2]string, t *ui.Theme, put func(x, y int, v uint32)) {
	if b.empty() {
		return
	}
	rim := rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.4))
	fill := rgb(canvas.RGB(0xff, 0xf6, 0xe4))
	for y := b.y0; y < b.y1; y++ {
		for x := b.x0; x < b.x1; x++ {
			switch {
			case inRound(x, y, b.x0+1, b.y0+1, b.x1-1, b.y1-1, 2):
				put(x, y, fill)
			case inRound(x, y, b.x0, b.y0, b.x1, b.y1, 3):
				put(x, y, rim)
			}
		}
	}
	// Tail: a small wedge at mouth height pointing at the dog.
	ty := b.y0 + 9
	for i := range 3 {
		for j := -2 + i; j <= 2-i; j++ {
			x := b.x1 - 1 + i
			if tailLeft {
				x = b.x0 - i
			}
			v := fill
			if j == -2+i || j == 2-i {
				v = rim
			}
			put(x, ty+j, v)
		}
	}
	ink := []uint32{rgb(alarmInk(t)), rgb(ui.Mix(t.BG, canvas.RGB(0, 0, 0), 0.2))}
	for li, s := range lines {
		y := b.y0 + 3 + li*9
		x := b.x0 + 4
		for _, ch := range s {
			g, ok := font[ch]
			if !ok {
				g = font['?']
			}
			if li == 0 { // bold
				for gy := range g {
					g[gy] |= g[gy] >> 1
				}
			}
			for gy, row := range g {
				for gx := range Tile {
					if row&(0x80>>gx) != 0 {
						put(x+gx-1, y+gy, ink[li])
					}
				}
			}
			x += 6
			if li == 0 {
				x++
			}
		}
	}
}

// alarmInk is the hottest color of the theme's ramp that still reads on
// the bubble's cream fill (some themes end the ramp near white), darkened
// as a last resort.
func alarmInk(t *ui.Theme) canvas.Color {
	lum := func(c canvas.Color) float64 { return 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B) }
	for v := 1.0; v >= 0.6; v -= 0.05 {
		if c := t.Heat(v); lum(c) < 150 {
			return c
		}
	}
	c := t.Heat(1)
	for range 8 {
		if lum(c) < 150 {
			break
		}
		c = ui.Mix(c, canvas.RGB(0, 0, 0), 0.18)
	}
	return c
}
