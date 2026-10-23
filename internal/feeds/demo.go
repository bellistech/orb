package feeds

import "time"

// Demo returns plausible feed data for -demo and screenshots.
func Demo(now time.Time) View {
	ago := func(m int) time.Time { return now.Add(-time.Duration(m) * time.Minute) }
	cat := func(name string, items ...Item) Category { return Category{Name: name, Items: items} }
	n := &News{Generated: now, Categories: []Category{
		cat("Security",
			Item{"Weekly Recap: Helpdesk 0-Days, Rogue Certs and AI-Fooling Malware", "The Hacker News", ago(25), "Some meetings last longer than it took attackers to jump on a newly disclosed flaw this week. Within hours of the advisory, scanning began across the internet, and by the next morning working exploits were circulating. Helpdesk software was the soft target again: two zero-days let attackers reset passwords and enroll their own MFA devices. Meanwhile a certificate authority revoked thousands of certificates after finding a validation bug, and researchers showed malware that hides from AI-based detection by padding itself with harmless-looking code."},
			Item{"OpenSSH 10.4 released with fixes for two client bugs", "LWN.net", ago(140), ""},
			Item{"CISA adds three exploited VPN flaws to the KEV catalog", "CISA Advisories", ago(300), ""},
			Item{"Satel Netco Design", "CISA Advisories", ago(2900), ""},
			Item{"Inside a ransomware crew's build pipeline", "Krebs on Security", ago(4000), ""}),
		cat("Networking",
			Item{"Sample netlab SRv6 Lab Topologies", "ipSpace.net", ago(48), "With the growing number of SRv6 implementations that netlab supports, it made sense to create a set of topologies you can use to explore them. Each lab starts small, a pair of routers with a single locator, and builds up to L3VPN services across a small core. The labs run on containers where the vendor allows it, so you can bring them up on a laptop in a couple of minutes and tear them down just as fast."},
			Item{"EVPN multihoming without the tears", "ipSpace.net", ago(600), ""},
			Item{"BGP route leaks of the month", "APNIC Blog", ago(1500), ""},
			Item{"QUIC load balancing in practice", "Cloudflare", ago(3000), ""}),
		cat("SRE / DevOps",
			Item{"Hacking the Go compiler to efficiently map IPv4 to IPv6", "Vincent Bernat", ago(90), "netip.Addr features an Unmap() method returning the unwrapped IPv4 address contained in an IPv4-mapped IPv6 address. Going the other way is surprisingly slow, because the compiler cannot see through the representation. A small change to how the compiler handles the 128-bit value turns a dozen instructions into two, which matters when you convert addresses millions of times per second in a flow collector."},
			Item{"Error budgets for batch jobs", "Google SRE", ago(2000), ""},
			Item{"What a 3am page taught us about retries", "Honeycomb", ago(5000), ""}),
		cat("Linux",
			Item{"ROCm 10 OpenCL vs. Mesa Rusticl on AMD Strix Halo", "Phoronix", ago(30), ""},
			Item{"[$] The 2026 Maintainers Summit", "LWN.net", ago(150), "The Maintainers Summit is a one-day, invitation-only gathering of about 30 kernel developers; it is held once a year alongside the Linux Plumbers Conference. This year's discussions covered maintainer burnout, the growing use of AI tools to generate patches, and how the Rust experiment is going now that several drivers depend on it."},
			Item{"Linux 6.18-rc1 released", "LWN.net", ago(1300), ""}),
		cat("Community",
			Item{"A Modern Web App in C", "Hacker News", ago(12), ""},
			Item{"Show HN: A dashboard that looks like a SNES menu", "Hacker News", ago(70), "I wanted something on the big monitor above my desk that I could read from across the room, so I built a view-only dashboard that draws straight to the Linux framebuffer. It uses 8x8 tiles, a hand-drawn font and windows styled after 16-bit RPG menus, and it costs about a tenth of a millisecond per frame when nothing changes."},
			Item{"The quiet death of the RSS reader, again", "Hacker News", ago(800), ""}),
		cat("Other",
			Item{"King Gizzard announce winter tour dates", "KGLW", ago(200), "The band will play three nights in each city, with no repeated songs across the run, and a marathon set on the final night. Tickets go on sale Friday, and as usual a share of every ticket goes to environmental charities chosen by the band."},
			Item{"Opening Reception for Sandy Hook's Artist in Residence", "National Park Service", ago(9000), ""}),
	}}
	w := &Weather{
		Location: "Austin, Texas", Updated: ago(14),
		Now: Now{Temp: 80, High: 91, Low: 72, Precip: 40, Summary: "Scattered showers", Kind: Rain},
		Daily: []Day{
			{"Monday", 89, 72, "Scattered thunderstorms", Storm},
			{"Tuesday", 93, 77, "Partly sunny", PartlyCloudy},
			{"Wednesday", 90, 72, "Partly sunny", PartlyCloudy},
			{"Thursday", 89, 67, "Sunny", Clear},
			{"Friday", 70, 54, "Light rain", Rain},
		},
	}
	temps := []int{79, 81, 83, 85, 86, 86, 84, 82, 79, 77, 75, 74}
	kinds := []Kind{Cloudy, Cloudy, Rain, Storm, Storm, Rain, Cloudy, PartlyCloudy, PartlyCloudy, Clear, Clear, Clear}
	for i, tp := range temps {
		h := (11 + i) % 12
		if h == 0 {
			h = 12
		}
		ap := "AM"
		if i >= 1 && i < 13 {
			ap = "PM"
		}
		w.Hourly = append(w.Hourly, Hour{Label: itoa(h) + ap, Temp: tp, Kind: kinds[i]})
	}
	return View{News: n, Weather: w, NewsAt: now, WeatherAt: now, NewsOn: true, WeatherOn: true, Version: 1}
}

func itoa(i int) string {
	if i >= 10 {
		return string(rune('0'+i/10)) + string(rune('0'+i%10))
	}
	return string(rune('0' + i))
}
