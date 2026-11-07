package drinks

import (
	"strings"
	"testing"
	"time"

	"coffee-and-toe-beans/content"
)

func menu(t *testing.T) *Menu {
	t.Helper()
	m, err := Load(content.FS, "drinks.json")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func clock(hh, mm int) time.Time {
	return time.Date(2026, 11, 7, hh, mm, 0, 0, time.Local)
}

func TestTheMenu(t *testing.T) {
	m := menu(t)
	want := []string{"cafe_creme", "chocolat_chaud", "chai_latte", "earl_grey", "matcha_latte", "vin_chaud", "tisane_verveine"}
	if len(m.Drinks) != len(want) {
		t.Fatalf("menu has %d drinks, want %d", len(m.Drinks), len(want))
	}
	for i, id := range want {
		if m.Drinks[i].ID != id {
			t.Errorf("drink %d is %s, want %s", i+1, m.Drinks[i].ID, id)
		}
	}
}

func TestServingHours(t *testing.T) {
	m := menu(t)
	vin, _ := m.Get("vin_chaud")
	tisane, _ := m.Get("tisane_verveine")
	creme, _ := m.Get("cafe_creme")
	cases := []struct {
		hh, mm      int
		vin, tisane bool
	}{
		{9, 0, false, false},
		{16, 59, false, false},
		{17, 0, true, false},
		{20, 59, true, false},
		{21, 0, true, true},
		{0, 30, true, true},
		{1, 59, true, true},
		{2, 0, false, true},
		{5, 59, false, true},
		{6, 0, false, false},
	}
	for _, c := range cases {
		at := clock(c.hh, c.mm)
		if vin.Available(at) != c.vin {
			t.Errorf("%02d:%02d vin chaud available = %v", c.hh, c.mm, !c.vin)
		}
		if tisane.Available(at) != c.tisane {
			t.Errorf("%02d:%02d tisane available = %v", c.hh, c.mm, !c.tisane)
		}
		if !creme.Available(at) {
			t.Errorf("café crème should always be served")
		}
	}
}

func TestACupCoolsAndEmpties(t *testing.T) {
	m := menu(t)
	o := Order{Drink: "cafe_creme", At: clock(10, 0), Seed: 42}
	at := func(min float64) time.Time { return o.At.Add(time.Duration(min * float64(time.Minute))) }

	if m.Level(o, at(0)) != 1 || m.Heat(o, at(0)) != 1 {
		t.Error("a new cup is full and hot")
	}
	if m.Level(o, at(m.Pace.FirstSip-0.01)) != 1 {
		t.Error("nobody sips before the first sip time")
	}
	prevLevel, prevHeat := 1.0, 1.0
	sawSteps := 0
	for min := 0.0; min <= 20; min += 0.25 {
		l, h := m.Level(o, at(min)), m.Heat(o, at(min))
		if l > prevLevel || h > prevHeat {
			t.Fatalf("at %.2f min the cup got fuller or hotter (%.2f, %.2f)", min, l, h)
		}
		if l < prevLevel {
			sawSteps++
		}
		prevLevel, prevHeat = l, h
	}
	if sawSteps < m.Pace.Sips/2 {
		t.Errorf("the cup should empty in sips, saw %d steps", sawSteps)
	}
	if m.Empty(o, at(14.9)) || !m.Empty(o, at(15)) {
		t.Error("empty at exactly 15 minutes")
	}
	if m.Level(o, at(15)) != 0 || m.Heat(o, at(15)) != 0 {
		t.Error("an empty cup has nothing left and no steam")
	}
	if h := m.Heat(o, at(6)); h <= 0.1 || h >= 0.9 {
		t.Errorf("halfway through it should be warm, not hot or cold: %.2f", h)
	}
	if h := m.Heat(o, at(12.5)); h != 0 {
		t.Errorf("cold by cool_minutes, got %.2f", h)
	}
	if !m.EmptySince(o).Equal(at(15)) {
		t.Error("EmptySince should be the finish time")
	}
	if m.Level(o, at(-5)) != 1 {
		t.Error("a time before the order counts as just arrived")
	}
}

func TestEveryCupIsALittleDifferent(t *testing.T) {
	m := menu(t)
	a := Order{At: clock(10, 0), Seed: 1}
	b := Order{At: clock(10, 0), Seed: 99999}
	differ := false
	for min := 0.0; min < 15; min += 0.1 {
		at := a.At.Add(time.Duration(min * float64(time.Minute)))
		if m.SipsTaken(a, at) != m.SipsTaken(b, at) {
			differ = true
		}
		if m.SipsTaken(a, at) != m.SipsTaken(a, at) {
			t.Fatal("the same cup must always give the same answer")
		}
	}
	if !differ {
		t.Error("two cups should be sipped at slightly different moments")
	}
}

func TestParseErrors(t *testing.T) {
	good := `{"pace": {"first_sip_minutes": 2, "finish_minutes": 15, "cool_minutes": 12, "sips": 8},
	  "cups": {"c": {"sprite": "s"}},
	  "drinks": [DRINK]}`
	drink := `{"id": "x", "name": "X", "icon": "i", "cup": "c", "color": "#000000", "top": "#ffffff", "served": ["Here."]FIELD}`
	cases := map[string]string{
		`, "cup": "nope"`:                     "isn't in \"cups\"",
		`, "color": "brown"`:                  "color",
		`, "from": "17:00"`:                   "both \"from\" and \"until\"",
		`, "served": []`:                      "\"served\"",
		`, "name": ""`:                        "no \"name\"",
		`, "until": "25:99", "from": "17:00"`: "until",
	}
	if _, err := Parse("ok.json", []byte(strings.Replace(good, "DRINK", strings.Replace(drink, "FIELD", "", 1), 1))); err != nil {
		t.Fatalf("the good menu should load: %v", err)
	}
	for field, want := range cases {
		src := strings.Replace(good, "DRINK", strings.Replace(drink, "FIELD", field, 1), 1)
		_, err := Parse("bad.json", []byte(src))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want an error mentioning %q", field, err, want)
		}
	}
	twice := strings.Replace(good, "DRINK", strings.Replace(drink, "FIELD", "", 1)+","+strings.Replace(drink, "FIELD", "", 1), 1)
	if _, err := Parse("bad.json", []byte(twice)); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("a drink listed twice should be refused: %v", err)
	}
}
