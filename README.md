# Progress — Coffee and Toe Beans

Read this first at the start of every milestone conversation. The design is in
`gameplay.md` and the plan in `brief.md` (both in the claude.ai project).
Version 1 is finished and published (v1.0.0 steps below). This file records
what version 1 is, the decisions behind it, and where version 2 starts.

Where an older note conflicts with a later one, the later one wins. Current
pacing is in the "Pacing" section below.

## Status

| Milestone | State |
|-----------|-------|
| M1 — The room | Done; review changes applied (7 Nov 2026) |
| M2 — Talking | Done; review changes applied (7 Nov 2026) |
| M3 — The cats | Done (7 Nov 2026) |
| M4 — The narrator and memory | Done; reviewed by Nala (7 Nov 2026) |
| M5 — Comfort and polish | Done (8 Nov 2026). Ready to publish **v1.0.0** |
| Test fixes | Done (12 Nov 2026): three stale test expectations and one test setup (see below). Not yet committed |

Version 1 is complete. Next: Nala publishes v1.0.0, then version 2.

## Pacing (current)

Set in `content/pacing.json`; the Settings screen and README describe it.

- **Cat moments:** every **5–10** café minutes (`moment_minutes: [5, 10]`).
  Changed by Nala on 8 Nov 2026 (was 10–20). Always wait for an answer.
- **Very slow** (Settings): 1.5× the waits, so moments every 7½–15 minutes.
- **Quiet mode** (Settings): halves both moments and small talk, so moments
  every 10–20 minutes and small talk every 16–30.
- **Small talk:** every 8–15 café minutes (`small_talk_minutes`), never
  within a café minute of anything else.
- **Cup offer ("Another?"):** 1–3 café minutes after a cup empties, once.
  It waits behind any cat moment on screen (moments always block other talk).
- **Looking at a cat** (Look around) adds a little bond once every 5 café
  minutes.

Note: the comment in `content/pacing.json` still says Very slow means
"every 15–30 minutes". That is stale; it should read 7½–15. Fix it with the
next content change.

## Version 1 in short

A cosy, silent cat café in the terminal (Go + Bubble Tea, one ~4 MB program,
macOS and Linux, Windows through WSL).

- **The room:** the French interior (104 × 84 pixels, 104 × 42 cells), the
  view over Paris with the Eiffel Tower (lit at dusk, sparkling on the hour),
  lamps, candle, bookcase, chalkboard; title card.
- **Time and weather:** real-clock lighting following Paris's real sunrise and
  sunset through the seasons; ambient weather made from the date (clear,
  cloudy, drizzle, rain, heavy rain, fog, winter snow). Both can be chosen in
  Settings.
- **Talking:** a visual-novel dialogue box (typing, ▼, choices), the Esc menu
  (Order a drink · Look around · Settings · Leave the café), and the drinks
  chalkboard with seven drinks in seven cups that cool and empty over 15
  minutes.
- **The cats:** Brie, Hunter and Mimi, drawn from Nala's PixelLab art; they
  wander, sleep, blink and flick their tails; hidden bonds with five tiers;
  58 moments each; Look around to look at and interact with them; Mimi's
  patience.
- **The narrator and memory:** welcomes by time and weather, small talk,
  "while you were away" notes, your favourite drink; autosave, safe writes
  and broken-save recovery.
- **Comfort:** Settings for reduced motion, quiet mode, text speed, pacing,
  weather, time of day and colours; true colour or a hand-tuned 256-colour
  look for macOS Terminal.
- **Release:** README with a GIF, CI that tests and builds every push,
  GoReleaser releases with four clearly named zips.

## Decisions made (and why)

Recorded so later work doesn't reopen them without reason.

1. **Content is JSON, art is text grids.** YAML would need an outside library,
   which would stop the content code being testable here. Each JSON file starts
   with an `"_about"` note. Sprite format: `content/sprites/README.md`.
2. **Our own renderer.** Every pixel and letter is drawn by our standard-library
   canvas. Only Bubble Tea is imported (pinned at v1.3.4).
3. **Module name `coffee-and-toe-beans`**, not a GitHub path.
4. **256-colour fallback:** perceptual (Oklab) matching, harsh single-channel
   colours left out. Dithering looked like noise on the walls. macOS Terminal
   is detected as 256-colour.
5. **Seasonal light:** keyframes describe a model day (sunrise 07:30, sunset
   19:30); the real clock is mapped onto it with Paris's actual sunrise and
   sunset for the date.
6. **Weather from the date**, so relaunching keeps the same sky.
7. **Esc menu:** "Leave the café" is the first choice, so Esc then Enter leaves.
8. **Scene fills the whole frame** (104 × 42 cells). Dialogue box sits bottom-left.
9. **CI from M1** (`.github/workflows/ci.yml`, Linux + Mac).
10. **Golden frames use a tiny test café** in `internal/scene/testdata`, so
    editing the real art never breaks tests. After an intended drawing change:
    `go test ./internal/scene -update`.
11. **Cat spots** are defined in the interior file (sill, shelf top, sideboard,
    cushion, table, floor).
12. **Extension points:** interiors are data files built from reusable element
    types; weather kinds are data; cups and drinks are data.
13. **Conversations are data** (`content/dialogue/*.json`). Pages, choices with
    an action (`"do": "order"`), and the choice taken when ignored.
14. **Dialogue timing uses real seconds, not café time**, so `--fast` speeds up
    the café but never makes words unreadable.
15. **A cup is calculated from when it was ordered**, never ticked along.
16. **Cups are sprites with `@drink` pixels**; colours come from
    `content/drinks.json`.
17. **Settings apply live, never by restarting** (`applySettings`).
18. **Time of day is a look, not a clock.** A chosen time changes what the café
    shows, but the café clock keeps running, so cups, moments and saves behave
    the same whatever is chosen.
19. **256 colours: leave out loud colours rather than dither.** Cats are matched
    separately so they keep their colours.
20. **Release files have no version in their names**, so "latest download"
    links in the README never go stale.
21. **Saving** goes through `Config.Store`, so tests use an in-memory store.
    `internal/save` never panics on a bad file.
22. **`--fast` and `--time` use `save-test.json`**, so testing never touches the
    real café.
23. **Tunable timings live in `content/pacing.json`**: moments, small talk,
    narrator question wait (90 s), cup-kept window (30 min), favourite
    threshold (3 orders), autosave (60 s).
24. **The save keeps wall-clock time for visits** and café time for seen
    moments. The cup is kept as ages, not clock times.

## Test notes (12 Nov 2026)

Three tests failed after the 8 Nov pacing change. Fixed; not yet committed.

- `internal/cats` `TestPacing`: expected `[10, 20]`; now expects `[5, 10]`.
- `internal/game` `TestTheCatsAndTheNarratorTakeTheirTime`: moment bound is
  now `180/5` (the 5–10 minute cadence), not `180/10`.
- `internal/game` `TestTheCupCoolsEmptiesAndStays`: the test now answers any
  open cat moment before waiting for the "Another?" offer. The game was right
  to hold the offer behind the moment; the test never answered the moment.

Changed files: `internal/cats/cats_test.go`, `internal/game/cats_test.go`,
`internal/game/talk_test.go`.

`go test ./...` passes except `cmd/coffee-and-toe-beans` and `internal/ui`,
which need Bubble Tea from `proxy.golang.org`. That can't be reached from the
sandbox; they pass on Nala's machine.

## Open questions for Nala

- After playing v1.0.0 on the Mac: does the 256-colour café look right (sunset,
  evening, night), and is the 5–10 minute pacing right?
- Support links: which platforms (GitHub Sponsors, Ko-fi, itch.io), so the
  README and `.github/FUNDING.yml` can be filled in.
- Table close-ups (see version 2) when PixelLab is available again.

## Changes after the first M1 review (7 Nov 2026)

- The café is called **Coffee and Toe Beans**. Nala still owns it (and is still
  never seen).
- **Title card:** the title in spaced italics between two thin rules, then
  "press Enter to step inside". No logo.
- **Palette "warm clay, charcoal wood"** (`content/interiors/french.json`).
- **Calmer surfaces** and **warmer nights** (blue-violet sky, dim rose-brown room).
- **Real seasons** via the model-day mapping above.
- Nala uses macOS Terminal (256 colours). Tip given: line spacing 1.0.

## M5: comfort and polish (7–8 Nov 2026)

- **Esc menu:** Order a drink · Look around · **Settings** · Leave the café.
  The title card's menu is **Settings · Leave the café**.
- **Settings screen** (`internal/game/settings.go`; words in
  `content/text/ui.json` → `settings`): Reduced motion, Quiet mode, Text speed
  (Slow / Normal / Instant), Pacing (Gentle / Very slow), Weather, Time of day,
  Colours (Auto / True colour / 256). ↑ ↓ choose, ← → change, Enter = →, Esc
  back. Every change applies at once and is saved at once. On the title card
  the visit isn't counted.
- **Reduced motion:** no lamp or candle flicker, no tower sparkle or star
  twinkle, sparser and slower rain and snow, no blinking ▼ or pulsing title,
  shorter fades.
- **Quiet mode:** half as much small talk and half as many cat moments
  (tested: 36 → 18 over 4 café hours). Very slow: 36 → 26.
- **Time of day:** each look is a time on the model day (`chosen_times` in
  `content/lighting.json`).
- **256 colours:** loud colours removed from the room (`loud` in
  `internal/rgb/xterm256.go`); dim cats have their fur tones lifted a little
  before matching.
- **Performance** (virtual terminal, 104 × 42): heavy rain at night uses 3.0 %
  of one core in 256 colours, 4.3 % in true colour. A 25-minute `--fast` run
  averaged 2.8 % CPU; memory stayed 14–18 MB.
- **Release setup:** `README.md` (GIF `docs/cafe.gif`, screenshot, controls,
  settings, save location, building from source, licence), CI, GoReleaser
  release workflow, `LICENSE` (MIT, natalielpx), `SPONSORS.md` and
  `.github/FUNDING.yml` placeholders. GoReleaser itself couldn't be run in the
  workspace; the first real run is Nala's tag push.

## M4: the narrator and memory (7 Nov 2026)

- **Cat moments always have choices and wait for you.** Nothing times out. No
  visible "pass" choice; a moment's `ignore` is only used, quietly, if you leave
  the café while it's open. A moment with no choices is refused at load.
- **Observations live in Look around**, not as pop-ups. A cat's panel shows its
  look line, then about half the time an observation that fits.
- **Narrator boxes** pass by themselves after ~7–10 s; a narrator question waits
  90 s (`question_seconds`).
- **Affirmations removed** (Nala's choice).
- **Small talk** (`content/narrator/small_talk.json`, 36 lines), every 8–15 café
  minutes, never during a cat moment; a line isn't repeated until about 70 % of
  the others have been used.
- **Coming back** is one box: welcome, then the away note, then the drink offer,
  or a line about your drink if it's still there (`cup_kept_minutes`: 30).
- **Favourite drink** (ordered at least 3 times, no tie) is only mentioned.
- **Welcome lines** (31), **away notes** (opener by gap, one cat line by bond and
  length, half the time a second page for a day or more), all in `content/`.
- **Saving:** `os.UserConfigDir()/coffee-and-toe-beans/save.json`. Autosave every
  60 real seconds inside the café, and on leaving, Ctrl+C or closing the terminal.
  Written to a temporary file, flushed, then renamed. A broken save is moved to
  `save.broken-<date>-<time>.json`, the café starts afresh, and a note is printed
  after leaving. A save from a newer version is copied first.
- **Hidden flags:** `--fresh`, `--away 3d`.
- **Fix after Nala's first play:** each cat offers one action per Look around;
  rest and descriptions stay the same within one visit to the panel.
- **Look around list capped:** at most 4 actions per cat, at most one sudden.
- **150 more cat moments** (10 per cat per bond level, 3–5 choices each), built
  from Nala's doc with a parser. Each cat now has 58 moments.
- **British spelling** in everything the player reads.

## M3: the cats (7 Nov 2026)

- **Art:** Brie, Hunter and Mimi drawn from Nala's PixelLab references
  (24 × 24 canvases). Sitting, standing, loaf and asleep poses; little movements
  (blink, ear twitch, tail flick, breathing asleep). Art in
  `content/sprites/cat_<id>.txt`; each cat in `content/cats/<id>.json`.
- **Wander** between the six spots, fading in and out. Never two at one spot, never
  overlapping, never hanging off an edge. A little warm light after dark.
- **Follow the hour:** sleepy at night, playful in the evening.
- **Bonds:** hidden, 0–100, tiers at 10 / 25 / 45 / 70 (stranger, curious,
  familiar, friend, your cat).
- **Mimi:** never goes near a stranger on her own; calm choices raise her bond,
  sudden ones lower it and she leaves (enforced in code too).
- **Look around:** Esc menu entry. In Look around the cats hold still; ← → move a
  gold arrow between the window, each cat and the table. A stranger gets a
  description only.
- **Moments** (`content/moments/<id>.json`): dialogue format plus conditions and
  effects. Ignoring always takes a quiet choice.
- **Snapshot tool:** `--cat`, `--bond`, `--moment`, `--look`, `--screen`.

## M3 step 1: the bigger room (7 Nov 2026)

- The café went from 80 × 64 to **104 × 84 pixels (104 × 42 terminal cells)**,
  laid out by hand, not stretched.
- All furniture, cups and decor redrawn about 1.3× in the same style.
- The view of Paris keeps its pixel scale; rain and snow scale with the glass area.
- **Screens:** title card re-centred; pause menu in the middle of the screen;
  drinks menu centred; too-small message asks for 104 × 42.
- **TestPhases** uses fixed Paris time, so it passes on UTC machines.

## M2: talking (7 Nov 2026)

- **Dialogue box** low on the left. Letters type out (about 34 a second); Enter
  shows the page at once, then turns it. Choices (2–4), ↑ ↓ to choose, Enter to
  confirm.
- **Nothing waits forever** for narrator lines; cat moments always wait.
- **The hello:** a couple of seconds after stepping inside, the narrator greets you
  and offers a drink.
- **Drinks** follow the café clock: vin chaud 17:00–02:00, tisane verveine
  21:00–06:00, the rest all day. A new drink replaces the cup even if unfinished.
- **Seven cups** (café crème, chocolat chaud, chai latte, Earl Grey, matcha latte,
  vin chaud, tisane verveine). Cooling: steam full for a minute, gone by 12 min.
  Sips empty the cup at exactly 15 minutes.
- **Empty cup:** one offer, picked from `cup_empty*` conversations, never the same
  twice in a row (`content/dialogue/cafe.json`).

## M1: the room (7 Nov 2026)

- **Title card**, the French interior, the view over Paris with the Eiffel Tower
  (golden at dusk, sparkling on the hour until 23:00), real-clock lighting
  (`content/lighting.json`), ambient weather from the date, steam from the cup.
- **Colour:** true colour, or the hand-tuned 256-colour fallback, detected
  automatically (override with `--color`).
- **Resizing:** the café stays centred in bigger windows; smaller windows get a
  polite message.
- **Flags:** `--time HH:MM`, `--fast` (30×), `--version`, `--weather KIND`,
  `--color auto|truecolor|256`.
- **Snapshot tool:** `go run ./cmd/snapshot --time 19:00 --weather rain`.

## How the code is laid out

```
cmd/coffee-and-toe-beans/  the game: reads flags, starts the UI
cmd/snapshot/              dev tool: prints one still frame (any screen, any drink)
content/                   ALL art, words, lighting and weather (embedded)
docs/                      the README's GIF and screenshot
.github/, .goreleaser.yaml CI on every push; releases when a v* tag is pushed
internal/ui/               the only code that uses Bubble Tea (thin)
internal/game/             screens, keys, clock, layout of a frame
internal/scene/            the room, the view, weather effects, lighting
internal/canvas/           half-block pixel canvas → terminal text
internal/rgb/              colours and the 256-colour matching
internal/sprites/          reads the pixel-art text files
internal/dialogue/         the dialogue box: typing, pages, choices, ignoring
internal/drinks/           the menu, serving hours, cooling and sipping
internal/cats/             the cats: who they are, wandering, bonds, moments, pacing
internal/narrator/         welcome, away notes and small talk from content/narrator
internal/save/             the save file: load, safe write, broken-save backup, favourite
internal/timeofday/        lighting keyframes, moon phase
internal/weather/          the ambient weather story
internal/text/             loads on-screen words
internal/noise/            repeatable random patterns
```

## Building and testing

- Standard checks: `gofmt -l .`, `go vet ./...`, `go test ./...`.
- In the sandbox, the full build needs Bubble Tea from GitHub, since
  `proxy.golang.org` is blocked. Clone the `golang.org/x/...` modules at old tags
  into a **separate** dev module file and use `-modfile=dev.mod`. Never ship those
  replace lines; the real `go.mod` stays as it is.
- To play in a virtual terminal: run the binary in a pseudo-terminal (Python `pty`
  + `pyte`, at 104 × 42). Use `XDG_CONFIG_HOME` and `HOME` pointing at a scratch
  folder so test plays don't touch a real save.
- Snapshot previews: `go run ./cmd/snapshot ...` prints ANSI.

## Next: version 2

From gameplay.md, roughly in this order:

1. **Rustic vintage and outdoor garden interiors** (data files, like
   `content/interiors/french.json`), and an Interior setting.
2. **The cat journal:** a page per cat with a portrait, a description that grows
   with the bond, and memories; unmet cats as silhouettes.
3. **Real weather** for a chosen city via Open-Meteo (optional, off by default,
   never required). Weather of choice is already done.
4. **More moments** (8–10 more per cat) and **shared two-cat moments**.
5. **Bigger cats on your table** (close-ups, 36 × 36 canvas, from Nala's PixelLab
   art). Plan: a `close_up` pose set in each cat file and `"close_up": true` on
   the table spot.
6. Also worth doing: an itch.io page for the same zips, a Homebrew tap, signing and
   notarizing the Mac build (needs an Apple Developer account).

Then version 3: seasons and festivities, seasonal drinks and decorations; later,
sponsors (thanks page, commissioned cats).

Known, not fixed: 256 colours at night, cats in dim corners turn greyish like the
rest of the room.

## Publishing a release (v1.0.0 and later)

In the repo folder, after the milestone's files are in and pushed:

```sh
git tag -a v1.0.0 -m "Coffee and Toe Beans 1.0.0"
git push origin v1.0.0
```

GitHub → Actions → "Release" runs (a few minutes), then the Releases page has the
four zips. To redo a tag after a fix: `git tag -d v1.0.0 && git push --delete
origin v1.0.0`, then tag again. The next version is `v1.0.1` (fixes) or `v1.1.0`
(new things).

## Notes for the next conversation

- **The code comes from the zip Nala attaches to the conversation** (e.g.
  `coffee-and-toe-beans-m5.zip`). Unzip it into its own folder, then read this file.
  The repo is public at `natalielpx/coffee-and-toe-beans` and can be cloned for
  reading.
- The project docs (`claude/PROGRESS.md`, `claude/brief.md`, `claude/gameplay.md`)
  live in the claude.ai project; this file is also in the zip.
- Python's `pyte` isn't installed by default: `pip install --break-system-packages
  pyte` before playing in the virtual terminal.
- Downloading programs such as GoReleaser into the workspace isn't allowed; check
  the release config against its documentation instead.
