# Progress — Coffee and Toe Beans

Read this first at the start of every milestone conversation. The design is in
`gameplay.md` and the plan in `brief.md` (both in the claude.ai project).

## Status

| Milestone | State |
|-----------|-------|
| M1 — The room | Done; review changes applied (7 Nov 2026) |
| M2 — Talking | Done; review changes applied (7 Nov 2026) |
| M3 — The cats | Done (7 Nov 2026): bigger room, the three cats, wandering, moments, bonds, Look around |
| **M4 — The narrator and memory** | **Done** (7 Nov 2026): welcomes, away notes, small talk, saving, the text pass. Waiting for Nala's review |
| M5 — Comfort and polish | |

## M4: the narrator and memory (7 Nov 2026)

Decided with Nala before building (all words approved in the review doc
"Milestone 4 words — for Nala's review", with her edits):
- **Cat moments always have choices and wait for you.** Every page waits
  for Enter, the cat stays put (held), nothing times out. There is **no
  visible "pass" choice**: each moment's `ignore` is only used, quietly, if
  you leave the café (Esc menu or Ctrl+C) while it's open. A moment with no
  choices is refused when the game loads.
- **No-choice cat lines live in Look around.** Observations ("Brie flicks
  an ear.") no longer pop up; a cat's Look around panel shows its look line,
  then about half the time an observation that fits (pose, spot, weather,
  hour) instead of the "doing" line, if it fits the 4-line panel. The six
  moments that had no choices got choices (brie_wash, brie_purr,
  hunter_dash, hunter_yourcat, mimi_night, mimi_lap).
- **Narrator boxes still pass by themselves**: pages after ~7–10 s as
  before; a narrator question now waits **90 s** (was 40), set by
  `question_seconds` in `content/pacing.json`.
- **Affirmations removed** (Nala's choice). gameplay.md and brief.md in the
  project were updated to match.
- **Small talk** on its own timer, every 8–15 café minutes, never during a
  cat moment, never within a café minute of anything else; a line isn't
  repeated until about 70% of the others have been used (remembered across
  visits). Moments every 10–20 minutes as before. Quiet mode (M5) halves
  both (tested: 20 → 10 small talk lines in 4 hours).
- **Coming back** is one box: welcome, then the away note, then the drink
  offer ("Something to drink?"), or, if your drink is still there, a line
  about it instead.
- **Favourite drink** (the one ordered most, at least 3 times, no tie) is
  only mentioned: about half the time the offer becomes `arrive_favourite`
  ("Something to drink? Your usual café crème, maybe."), only if it's being
  served now. Same choices. The drinks menu opens on it.
- **British spelling** in everything the player reads (and in the notes in
  `content/`): favourite, colour, cosy, afresh. Field names like `color` and
  the `--color` flag stay as they are.

What's in:
- **Welcome lines** (`content/narrator/welcome.json`, 31): first visit (5),
  returning (24), and "regular" (2, after 5 hours in the café in total,
  mixed in). Picked by time of day and weather; lines with conditions are
  picked more often when they fit.
- **Away notes** (`content/narrator/away.json` + the `away` part of each
  `content/moments/<cat>.json`): an opener by gap (under an hour, a few
  hours, about a day, a few days, a week or more; 12 lines), then one cat
  line by how well that cat knows you (new / knows / yours) and how long you
  were gone (short under 10 h, long; 12 lines per cat). Usually the cat with
  the highest bond (about 2 in 3), sometimes another. For a day or more,
  half the time a second page with a small change in the café (11 lines).
  If opener and cat line don't fit one page together (only the two longest
  pairs), they take a page each. Bonds never drop while you're away.
- **Small talk** (`content/narrator/small_talk.json`, 36): café, street,
  Paris, and Nala (8 lines), tagged by time of day and weather.
- **Your cup on return**: back within 30 minutes (`cup_kept_minutes`), your
  drink is on the table, as full as you left it but cooler (the sipping
  clock was paused: `Order.Paused`); `cup_kept` says so, and no drink is
  offered. Later, or if the cup was empty, the table starts empty.
- **Saving** (`internal/save`): `os.UserConfigDir()/coffee-and-toe-beans/save.json`
  (Mac: `~/Library/Application Support/coffee-and-toe-beans/save.json`).
  Holds `version` (1), first and last visit (real time), visits, seconds in
  the café, bonds, seen moments, orders per drink, the cup (as ages, not
  clock times), recent small talk, and `settings` (reduced_motion, quiet,
  color, text_speed, pacing; empty until M5, but read already).
  Autosave every 60 real seconds inside the café (`autosave_seconds`), and on
  leaving, Ctrl+C, or the terminal closing. Written to a temporary file in
  the same folder, flushed, then renamed. A missing save is a first visit; a
  broken one (cut off, empty, not JSON, wrong types, no version) is moved to
  `save.broken-<date>-<time>.json`, the café starts afresh, and a note is
  printed after you leave. A save from a newer version is read and also
  copied first (`save.newer-…`). Leaving from the title card without
  stepping in saves nothing.
- **Hidden flags**: `--fresh` (moves the save to `save.before-fresh-….json`
  and starts as a first visit), `--away 3d` (pretend the last visit was
  45m / 3h / 2d / 1w ago). Runs with `--fast` or `--time` use their own
  `save-test.json`, so testing never touches the real café.
- Snapshot tool: `--away 3d` and `--page N` (with `--screen talk`).
- Help text (`--help`) says where the save lives.
- README: controls, the save file and how to start afresh, the new flags
  and content files.

Text pass (approved):
- brie_rain "In her opinion, Paris should be dry."; brie_creme "Pretend
  you haven't noticed"; hunter_toy "drops it at your feet"; Hunter's Look
  around friend line "Hunter, your friend. He looks at you as if you might
  be hiding a toy." (Nala had edited the old line to "hiding a toy"; her
  words were kept in the new line); Hunter asleep "stretched out as long as
  he can go". The hello's first page ("You sit down at the marble table…")
  became the welcome lines. Everything else was already plain and is
  unchanged.

Tests added: save round trip, five kinds of broken save, leftover temp
files, newer saves, old saves getting defaults, favourite, --fresh; narrator
words all fit the box, enough lines, Nala light, welcome by visit and
weather, gaps, away notes (opener, cat choice, second page about half the
time, right band and length), small talk conditions and no repeats, pacing;
game: first visit then coming back (bonds, seen moments, favourite, visits,
away note, menu on favourite), away notes for all five gaps, cup kept / not
kept / empty cup, autosave twice in 2 minutes, Ctrl+C saves, a cut-off save
starts fresh and saves again, --away, favourite mentioned about half the
time, nothing said while a moment waits, a moment waits 10 minutes on its
first page and closes gently on leaving, no cat line without choices pops
up, quiet mode halves the talk, observations in Look around about half the
time, and a two-hour `--fast` session saving to a real file with a leave
and return half-way.

**Fix after Nala's first play (7 Nov 2026):** in Look around, a cat's
actions came back as soon as its 4-minute rest was over (8 seconds at
`--fast`), so the same interaction could loop. Now each cat offers one
action per Look around: after the reply, its panel shows the description
and then a quiet observation about the cat (one that fits, doesn't mention
"you", and isn't already in the description; else its "doing" line), with
no more actions until you close Look around and open it again. Descriptions
also stay the same within one Look around when you move away and come
back. The rest line ("Mimi would like some quiet now.") now only shows if
you open Look around again while the cat is still resting.

**Also after Nala's play (7 Nov 2026):** the Look around panel always sits
bottom-left, like the dialogue box (it used to move low right or high left
to avoid hiding what you look at). A cat's moment shows its choices in a
random order each time (`Box.Shuffle`); results, effects and the quiet
answer still follow the order written in the file. The narrator's offers
keep their fixed order (Order a drink first).

**Look around list capped (7 Nov 2026):** a cat's actions add up over
the tiers, so friend-level Mimi offered 6 (2 of them sudden) and "your cat"
7. Now a cat offers at most 4 each time you look around, at most one of
them sudden, newest tiers first, in a random order; the list stays the same
while you look.

**150 more cat moments (approved 7 Nov 2026,** from the doc "More cat
moments — for Nala's review"): 10 for each cat at each bond level, each
only at exactly that level (`tier` = `max_tier`), with 3 to 5 choices
(the limit is now 5) and a gentle quiet answer. Each cat now has 58
moments, 12 to 18 possible at any one level (was 2 to 8). Moments where a
cat comes to the floor keep 4 choices at most, so the box never hides it.
Mimi as a stranger only appears on the bookcase or sideboard. Built
straight from the doc with a parser (it reads the When words, the
pages split by "/", and the bracket words: much closer 3, closer 2, a
little closer 1, no change 0, sudden −1 (Mimi −2 and she leaves), quiet =
the ignore answer, then … = what the cat does next). Tests: every moment
can happen somewhere, 10 per level per cat, words fit.

Played here in a virtual terminal (104 × 42): first visit (welcome, order,
Look around, leave with the goodbye; save written), straight relaunch
("You weren't gone long. Hunter dashes past your legs…", "Your chai latte
is still on the table."), `--away 3d` (away note, drink offer), Ctrl+C
saving, a save cut off at 120 bytes (fresh start, backup, note after
leaving), `--fresh`, `--fast` writing `save-test.json`, and three minutes at
`--fast` with moments waiting 25 s untouched, small talk in between, Look
around.

## Changes after the first M1 review (7 Nov 2026)

Decided with Nala:
- **The café is called Coffee and Toe Beans.** Nala still owns it (and is
  still never seen). "Nala's · Paris" is gone from the title card.
- **Title card:** just the words: "C o f f e e   a n d   T o e   B e a n s"
  in italics between two long thin rules, then "press Enter to step inside".
  The paw logo was removed. (Terminals can't change font size; italics show
  in macOS Terminal.)
- **Palette "warm clay, charcoal wood":** sand walls, peach trim and table,
  charcoal bookshelf and wainscot, teal curtains, real gold brass, warm oak
  floor; the sideboard keeps its brown wood. Colors are in
  `content/interiors/french.json`.
- **Calmer surfaces:** no grain on the walls, faint marble veins, an even
  floor without seams. The bookshelf and rooftops keep their detail.
- **Warmer nights:** the night sky is blue-violet and the room a dim
  rose-brown instead of grey, especially in 256-color mode.
- **Real seasons:** sunrise and sunset follow Paris for each date
  (calculated, no internet). The keyframes now describe a "model day" with
  sunrise at 07:30 and sunset at 19:30, and the real clock is stretched onto
  it. So 18:00 is night in December and still light in June.
- **Esc menu:** "Leave the café" is now the first choice, so Esc then Enter
  leaves. Revisit when the full pause menu arrives in M2 (it will hold
  "Order a drink" and "Settings" too).
- The Eiffel Tower keeps its hourly sparkle.
- License holder: natalielpx.
- Nala uses macOS Terminal (256 colors). Tip given: set line spacing to 1.0
  to remove thin lines between rows.
- The acceptance check "07:00, 13:00, 19:00 and 01:00 look different" became
  "dawn, day, dusk and night look visibly different" (seasons move the hours).

## M3 step 2: the cats (7 Nov 2026)

Drawn from Nala's three PixelLab references (`brie_sitting.png`,
`hunter_sitting.png`, `mimi_sitting.png`, 24 × 24 canvases) and approved
after several rounds.
- **Sitting** is her drawings pixel for pixel (15–17 × 18 px, a little
  bigger than the 13 × 15 first planned; they fit every spot).
- **Standing** (arriving, about to leave): compact, four slim legs.
- **Loaf** (lying, awake). Brie was made fluffier (tufts, belly fringe,
  slimmer body) after "fat instead of fluffy".
- **Asleep**, one pose each, from Nala's photos: Brie curled with her
  fluffy cream tail over her face (a darker outline, not darker fur, shows
  the tail; one closed blue eye shows); Hunter stretched out on his side,
  eyes closed as clear lines (26 px long: only fits the sill, the floor and
  your table, so elsewhere he loafs); Mimi with her head tucked in, only the
  back of her head and ears pointing down, paws and tail tip showing.
- **Little movements**: blink, ear twitch, tail flick, breathing asleep.
  Pink toe bean key in each palette.
- Art: `content/sprites/cat_<id>.txt`. Each cat: `content/cats/<id>.json`
  (name, poses with frames and anchor, habits under `ways`). A new cat =
  one sprite file + one cat file + one moments file, no Go changes.

What the cats do:
- **Wander** slowly between the six spots, fading in and out (standing a
  moment on arrival). Never two at one spot, never overlapping on screen,
  never hanging off an edge. Drawn at the right depth (the table, candle,
  book and cup stay in front). A little warm light after dark so cats in
  dim corners stay visible.
- **Follow the hour**: sleepy at night, playful in the evening. Brie
  lingers; Hunter comes often, stays briefly; Mimi visits rarely.
- **Pacing** (`content/pacing.json`): an observation every 1–3 café
  minutes, a moment every 4–8, random, never on top of other talk. In a
  3-hour test: a moment every ~6 min, an observation every ~2.5. Quiet mode
  (M5) halves it (`Config.Quiet`, tested). (Changed in M4: observations
  are in Look around, moments every 10–20 minutes and they wait.)
- **Moments** (`content/moments/<id>.json`): 8 per cat, the dialogue format
  plus conditions (tier, max_tier, when, weather, poses, spots,
  drink, comes_to) and effects (bond per choice, manner calm/sudden, then).
  The cat's name is on the box. Ignoring always takes a quiet choice.
- **Bonds**: hidden, 0–100, tiers at 10 / 25 / 45 / 70 (stranger, curious,
  familiar, friend, your cat). Raised by moments, by sharing the room, more
  by watching the cat's place. Closer spots open up with the tier ("near"
  in the interior file); cats that know you arrive sooner.
- **Mimi**: never goes near a stranger on her own; calm choices always
  raise her bond, sudden ones always lower it and she leaves (enforced in
  code too); watching the cat corner helps her most.
- **← → label** (first version): replaced after review by Look around,
  see below.
- Interior file: spots are now objects (`at`, `behind`, `flip`, `poses`,
  `near`, `place`); the table and ivy got an `id`. Layout and art unchanged.
- Snapshot tool: `--cat brie:sit@sill/blink`, `--bond mimi=friend`,
  `--moment brie_table`, `--look mimi` (Look around on a cat, the window
  or the table), `--screen poses|sheet`.
- Nala's text edits applied (brie_look, brie_wash, brie_table; brie_table
  now needs a drink on the table).

**Look around** (Nala's redesign after the review, 7 Nov 2026):
- Esc menu is now **Order a drink · Look around · Leave the café**
  (leaving: Esc, ↓, ↓, Enter). Outside Look around, ← → do nothing and
  the corner label is gone.
- In Look around the cats hold still (they still blink and breathe; a cat
  already fading finishes). ← → move a small gold arrow between the
  window, each cat and your table, left to right (a white arrow with a
  black outline, chosen by Nala; colors in `content/text/ui.json`
  `look_marker` / `look_marker_outline`); it starts on the first
  cat. A panel (low left; low right or high left if it would hide the
  target) shows a description and, with the bond, things to do. Esc
  closes it.
- A stranger gets a description only. Then 2 actions at curious (plus a
  quiet one while asleep), 2 more at familiar, 2 at friend (including
  inviting the cat onto your table), 1 at "your cat". Each shows a short
  reply; then the cat rests 4 café minutes. Mimi's lists include a sudden
  choice that lowers her bond and makes her leave.
- Looking at a cat quietly adds a little bond, once every 5 café minutes
  (shy cats more). This replaced "watching the cat corner".
- Spontaneous cat moments are rarer: every 10–20 minutes.
- Words: the `look` part of `content/moments/<cat>.json` (looks by tier,
  doing by pose, actions, rest) and `content/look.json` (window and table
  lines, spot words). Menu and hint words in `content/text/ui.json`.
- The ← → label code and `content/attention.json` were removed; the
  "attention" condition on moments is gone.

Not done, for later:
- **Bigger, more detailed cats on your table** (about 1.4×, sit, loaf and
  curled sleep). Nala wanted it; she'll make them with PixelLab when she
  can (36 × 36 canvas, one pose per go, the cat's sitting PNG as style
  image, its sitting/loaf/sleep/stand as references; Hunter curled, not
  stretched). A hand-made draft exists but was set aside. Plan: a
  `close_up` pose set in each cat file and `"close_up": true` on the table
  spot.
- 256 colors at night: cats in dim corners turn greyish like the rest of
  the room.

## M3 step 1: the bigger room (7 Nov 2026)

Asked by Nala so the cat sprites fit: the café went from 80 × 64 to
**104 × 84 pixels (104 × 42 terminal cells)**. Approved from preview images.
- **Laid out by hand**, not stretched: window centered (44 × 50, frame 3 px,
  transoms at 14 and 29), bookcase left (6 shelves) with ivy, chalkboard on a
  wall molding, sideboard with lamp and books, radiator under the sill, cat
  bed in the right floor corner, table across the bottom, cup just right of
  the dialogue box (`"cup": {"stand": [64, 82]}`).
- **Redrawn bigger, same style:** wall lamps, chalkboard (with a little pink
  paw), sideboard, table lamp, book stack, radiator, cat bed, candle, your
  book (with a ribbon), ivy; all seven cups about 1.3× with new steam points.
  The roof across the street is 44 px wide and centered.
- **View of Paris** keeps its pixel scale: the bigger window shows a little
  more city. Rain, raindrops and snow now scale with the glass area
  (tuned for the old 30 × 34 glass), so the weather looks as heavy as before.
- **Cat spots** (pixel under the middle of the cat): sill [51, 56],
  shelf_top [11, 13], **sideboard [97, 50] (new)**, cushion [93, 70],
  table [91, 81], floor [54, 69]. No radiator spot: it would overlap the
  sill. The floor spot can be brushed by a tall dialogue box; the others
  stay clear of it (tested).
- **Screens:** title card re-centered (`titleRow`), longer rules; dialogue
  box unchanged (low left); **pause menu moved to the middle** of the screen
  (centered at the bottom it sat half over the dialogue box); drinks menu
  centered; too-small message asks for 104 × 42.
- **TestPhases fixed:** it used the computer's own time zone, so it failed
  on UTC machines (GitHub). It now uses fixed Paris time (CET/CEST) and
  dates, including a winter and a summer evening.
- Snapshot tool: `--screen drinks` fixed (it still pressed ↓ twice from the
  old pause-menu order).
- Frame cost ~0.6 ms at 104 × 42 (well under 1% CPU at 10 fps).
- Known, not new: in 256-color mode, sunset and evening light show visible
  bands on the walls (the 80 × 32 room did the same). Can be softened later.

## Changes after the M2 review (7 Nov 2026)

Decided with Nala:
- Controls (typing speed, ▼, waiting times), serving hours and the empty
  table at the start: all good as they were.
- **Pause menu is now Order a drink · Leave the café.** "Stay a little
  longer" is gone (Esc closes the menu). Leaving is Esc, ↓, Enter; on the
  title card the menu only has Leave.
- **Plainer words.** The first drafts were "a bit cringey"; every line was
  rewritten to be simple and natural (the hello, the arrival lines, the
  offers). Choices are now "Order a drink / Not yet" and "… / Not now".
- **The offer after an empty cup varies.** The game picks one of the
  conversations whose name starts with `cup_empty`, never the same one twice
  in a row:
  - `cup_empty`: "Your café crème is finished." / "Another?"
  - `cup_empty_suggest`: a simple recommendation of another drink served
    right now, with "A chai latte, please" (served at once), "Something
    else" (opens the menu) and "Not now".
  - `cup_empty_quiet`: just a note that the cup is empty, no question.
  More can be added in `content/dialogue/cafe.json` with no code changes.
- Drinks can have a `short` name for sentences: Earl Grey is "cup of Earl
  Grey" ("Maybe a cup of Earl Grey next?").
- `.gitignore` also ignores `*.zip`, `*.test`, `*.out`.

## M2: what was first delivered (7 Nov 2026)

Decided with Nala before starting:
- Pause menu order: **Leave the café · Stay a little longer · Order a
  drink** (Leave stays first, so Esc then Enter still leaves). On the title
  card it's just Leave / Stay.
- **Settings are hidden** from the menu until M5.
- **Ctrl+C** stays as the emergency exit.
- **← →** do nothing visible yet (from M3 they're used in Look around).
- **Drinks follow the café clock**: vin chaud 17:00–02:00, tisane verveine
  21:00–06:00, the rest all day. A new drink replaces the cup even if it
  isn't finished. An empty cup stays on the table until you order again.
- All M2 words are first drafts, for Nala to review.
- **No sponsor work for now** (no `sponsor` field). The `SPONSORS.md` and
  `.github/FUNDING.yml` placeholders from M1 are untouched.
- The new palette and title card were approved on Nala's Mac.
- GitHub: `natalielpx/coffee-and-toe-beans`.

What's in:
- **Dialogue box** low on the left, like a visual novel, leaving the cup in
  view. Letters type out (about 34 a second) with a little breath after
  commas and full stops; Enter shows the whole page at once, then turns the
  page; a blinking ▼ says there's more. Choices (2–4) appear under the
  question; ↑ ↓ choose, Enter confirms. The box fades in and out.
- **Nothing waits forever**: a page left alone turns by itself after about
  7–10 seconds; a question left alone for 40 seconds resolves with its
  "ignore" choice (always the quiet one). Pressing any arrow gives you more
  time. While a menu is open, the box waits.
- **The hello**: a couple of seconds after stepping inside, the narrator
  greets you and offers a drink (Order a drink / Just sit for a while).
- **Pause menu** (Esc), as above. Esc again closes it.
- **Drinks menu**: "On the chalkboard today", seven drinks with pixel icons,
  names and short descriptions. Drinks not served yet are faded with
  "from 17:00" and are skipped by the arrows. Esc goes back to where you came
  from. Ordering puts the drink on the table and the narrator mentions it
  arriving.
- **Seven cups**: café crème in the green-banded bistro cup, chocolat chaud
  in an ochre stoneware mug, chai latte in a tall glass, Earl Grey in a
  rose-painted teacup, matcha latte in a French breakfast bowl, vin chaud in
  a glass mug with a slice of orange, tisane verveine in a glass cup.
- **Cooling and emptying**: steam is full for a minute, then fades away by
  12 minutes. The first sip waits 2 minutes; then 8 sips, each a little
  differently timed, empty the cup at exactly 15 minutes (you can see the
  level go down, best in the glasses). An empty cup keeps a faint ring.
- **Another?**: 1–3 café minutes after a cup is empty, the narrator offers
  another, once. Ignored, the empty cup simply stays.
- The table starts empty now (the café crème from M1 only arrives if you
  order it).
- Snapshot tool: `--screen drinks|talk|menu`, `--drink ID`, `--sipped MIN`.
- Tests: wrapping, typing speed and pauses, skipping, pages, choices,
  ignoring, pausing, content errors (dialogue and drinks), serving hours,
  cooling, sips, emptying from the top, pause menu order, the hello, ordering,
  just sitting, Esc going back, evening drinks, replacing a drink, every line
  fitting its box, and a full `--fast` café day that talks and orders.

## M1: what was first delivered (at the old 80 × 32 size)

- **Title card**: the café, dimmed, with rain still falling behind it; a
  pink toe-bean paw; "Coffee and Toe Beans", "Nala's · Paris", and a softly
  pulsing "press Enter to step inside". Enter fades you into the café
  (Enter again skips the fade).
- **The French interior** (80 × 64 pixels = exactly 80 × 32 cells): Haussmann
  cornice and wall moldings, bistro-green wainscot, oak chevron floor, a tall
  walnut bookcase (books generated, never the same twice on another seed) with
  ivy on top, a French window with brass espagnolette and dusty-rose linen
  curtains on a brass rod, two brass wall lamps, Nala's chalkboard menu, a
  walnut sideboard with a lamp and books, an old cast-iron radiator, a cat
  bed, and your marble bistro table with a candle, a book and a café crème.
- **The view**: sky with drifting clouds, stars and a moon in its real phase,
  a hazy far skyline with a dome, the Eiffel Tower's tip, zinc mansard roofs
  with dormers and chimney pots, a nearer roof, and the balcony rail.
  Distant windows light up one by one in the evening. The tower's golden
  lights come on at dusk, it sparkles for five minutes on the hour until
  23:00, its beacon pulses, and it goes dark at 23:45.
- **Real-clock lighting**: 11 keyframes (`content/lighting.json`) blended
  smoothly; the room is lit by daylight from the window by day and by the
  lamps and candle at night (warm pools, flicker, and faint reflections in
  the glass).
- **Ambient weather**: clear, cloudy, drizzle, rain, heavy rain, fog, and snow
  (December–February only). Each day's weather is made up from its date, so
  reopening the game shows the same sky. Changes fade over ~6 minutes. Rain
  streaks, raindrops that gather and run down the panes, drifting snow that
  settles on the roofs and in the corners of the panes, fog banks.
- **Steam** rising from the cup.
- **Color**: true color, or a hand-tuned 256-color fallback, detected
  automatically (override with `--color`).
- **Resizing**: the café stays centered in bigger windows; smaller than
  80 × 32 shows a polite message with the needed and current sizes.
- **Leaving**: Esc opens a small menu (Stay a little longer / Leave the café).
  Esc again closes it. Leaving fades out and prints a goodbye line.
  Ctrl+C still quits at once.
- **Flags**: `--time HH:MM`, `--fast` (30×), `--version`, plus two preview
  helpers: `--weather KIND` and `--color auto|truecolor|256`.
- **Snapshot tool**: `go run ./cmd/snapshot --time 19:00 --weather rain`
  prints one still frame (handy to compare hours quickly).
- **Tests** (all standard library, all passing here): colors and the 256
  palette, the half-block canvas, sprite parsing and error messages, lighting
  (the four acceptance times differ; no visible steps; midnight wraps), moon
  phase, weather (same date = same weather; mostly rain; snow only in winter;
  gentle fades; snow settles and melts), the scene (every hour × weather
  renders; night darker; lamps glow; tower schedule; reduced motion; drops
  stay on the glass), golden frames of a tiny test café, the game (title →
  café → menu → leave; no accidental quitting; Ctrl+C; resizing; clock flags;
  a full café day at `--fast` in both color modes), content loading, and the
  Bubble Tea glue. `gofmt` and `go vet` clean.

## How the code is laid out

```
cmd/coffee-and-toe-beans/  the game: reads flags, starts the UI
cmd/snapshot/              dev tool: prints one still frame (any screen, any drink)
content/                   ALL art, words, lighting and weather (embedded)
internal/ui/               the only code that uses Bubble Tea (thin)
internal/game/             screens, keys, clock, layout of a frame
internal/scene/            the room, the view, weather effects, lighting
internal/canvas/           half-block pixel canvas → terminal text
internal/rgb/              colors and the 256-color matching
internal/sprites/          reads the pixel-art text files
internal/dialogue/         the dialogue box: typing, pages, choices, ignoring
internal/drinks/           the menu, serving hours, cooling and sipping
internal/cats/             the cats: who they are, wandering, bonds, moments, pacing
internal/narrator/         picks the welcome, away notes and small talk from content/narrator
internal/save/             the save file: load, safe write, broken-save backup, favourite
internal/timeofday/        lighting keyframes, moon phase
internal/weather/          the ambient weather story
internal/text/             loads on-screen words
internal/noise/            repeatable random patterns
```

Changes from the brief's suggested structure: the Bubble Tea model lives in
`internal/ui` (as the working-environment section asks) and `internal/game`
is pure Go, so it's testable here. `rgb`, `canvas`, `noise` and `text` were
added. `narrator` and `save` arrived in M4 as the brief suggested; the
game's side of them (when to speak, what to save) is in
`internal/game/memory.go`. The dialogue box and menus are drawn by
`internal/game` (`widgets.go`); `internal/dialogue` only keeps their state.

## Decisions made (and why)

1. **Content is JSON, art is text grids.** YAML needs an outside library,
   which would stop the content code being testable here. Each JSON file
   starts with an `"_about"` note explaining its fields. Sprite format:
   `content/sprites/README.md`.
2. **Our own renderer; no Lip Gloss yet.** Every pixel and letter is drawn by
   our standard-library canvas, so all of it is tested here. Only Bubble Tea
   is imported (pinned at v1.3.4 in `go.mod`); its API use was checked against
   the v1.3.4 source. Lip Gloss/Bubbles will only be added if a later
   milestone truly needs them.
3. **Module name `coffee-and-toe-beans`**, not a GitHub path, so nothing
   depends on the GitHub username.
4. **256-color fallback**: perceptual (Oklab) matching, with the harsh
   single-channel colors (#870000, #00005f…) left out and hue weighted a
   little higher. Dithering was tried and looked like noise on the walls.
   Dark colors become soft greys in 256 mode; the warm lamp pools and the
   pink curtains carry the mood. macOS Terminal is detected as 256-color.
5. **Seasonal light** (changed after review): keyframes describe a model day
   (sunrise 07:30, sunset 19:30); the real clock is mapped onto it with
   Paris's actual sunrise and sunset for the date.
6. **Weather from the date.** No saving needed, and relaunching keeps the same
   sky. "Cloudy" and "heavy rain" were added as kinds (they'll be needed for
   the weather-of-choice setting in version 2 anyway) so changes feel natural.
7. **Esc opens a tiny leave menu** now ("Leave the café" first, as Nala
   asked); in M2 it grows into the full pause menu.
8. **The scene fills the whole frame** (80 × 32 then; 104 × 42 since M3). The M2 dialogue box will sit
   over the bottom-left, like a visual novel, leaving the cup visible.
9. **CI added early** (`.github/workflows/ci.yml`, Linux + Mac): it's the only
   full build besides Nala's Mac. Release automation stays in M5.
10. **Golden frames use a tiny test café** in `internal/scene/testdata`, so
    editing the real art never breaks tests. They allow tiny rounding
    differences between Mac and Linux processors. After an intended drawing
    change: `go test ./internal/scene -update`.
11. **Cat spots are already defined** in the interior file (sill, shelf top,
    cushion, table, floor) for M3.
12. **Extension points**: interiors are data files built from reusable element
    types (fill, panel, floor, bookshelf, window, curtain, table, sprite,
    shade); the view of Paris is shared by all interiors; weather kinds are
    data; the cup can be swapped (`SetCup`) and its steam turned down
    (`SetSteam`) for M2's drinks; reduced motion is already wired through the
    scene (setting arrives in M5).

13. **Conversations are data** (`content/dialogue/*.json`): pages (one text,
    or a list to pick from at random), choices with an action (`"do":
    "order"`), and which choice is taken when ignored (counted from 1).
    `{drink}` is filled in. The cats' moments in M3 will use the same format,
    with a `speaker` name shown on the box's border.
14. **Dialogue timing uses real seconds, not café time**, so `--fast` speeds
    up the café (cooling, offers) but never makes words unreadable.
15. **A cup is calculated from when it was ordered**, never ticked along: the
    same order always gives the same cup, and saving it in M4 only needs the
    drink and the time.
16. **Cups are sprites with `@drink` pixels**: the drink's colors come from
    `content/drinks.json` (`color` and `top`), so a new drink needs no new
    art if it uses an existing cup. Every cup stands on one spot in the
    interior (`"cup": {"stand": [55, 62]}`), whatever its height.
17. **Lip Gloss and Bubbles are still not used.** Everything is drawn by our
    own canvas, which is fully tested here.
18. **The full game can now be built here.** See the notes below: real
    Bubble Tea v1.3.4 is fetched from GitHub, so `internal/ui` is compiled,
    vetted and tested for real, and the game was played in a virtual
    terminal before handover (start, hello, choices, menus, leaving, Ctrl+C).

19. **The save keeps real (wall-clock) time for visits** and café time for
    seen moments. The cup is kept as ages ("arrived 6 minutes ago, 1 of them
    untouched"), so it means the same whatever the café's clock says.
20. **Saving is done by the game through `Config.Store`**, so tests use an
    in-memory store and previews save nothing; `main` gives it the real
    file. `internal/save` never panics on a bad file.
21. **Away lines live in each cat's moments file**, so a new (sponsored)
    cat brings its own and needs no Go changes.
22. **`--fast` and `--time` use `save-test.json`.** At `--fast` the cats
    warm up 30 times faster, which would quietly change the real café.
23. **The narrator question wait (90 s), cup window (30 min), favourite
    threshold (3 orders) and autosave (60 s) are in `content/pacing.json`**,
    with the small-talk timing, so they can be tuned without code.

## Open questions for Nala

- Table close-ups (see M3) when PixelLab is available again.
- How chatty the café feels now: small talk every 8–15 café minutes and a
  cat moment every 10–20. Both are in `content/pacing.json`.
- 256 colours at night: cats in dim corners turn greyish (from M3).

## Next: M5 — Comfort and polish

Settings screen in the Esc menu: reduced motion, quiet mode, text speed,
pacing, colour mode (the save already has a `settings` part and the game
already reads `reduced_motion` and `quiet` from it). Final polish pass on
animation details. README (GIF or screenshot, install, Support section),
CI for linux/macOS × amd64/arm64, GoReleaser and the first release.

## Notes for the next conversation

- **The code comes from the zip Nala attaches to the conversation** (e.g.
  `coffee-and-toe-beans-m4.zip`), not from GitHub: the repo isn't connected
  to these chats. Unzip it into its own folder, then read this file.
- The project docs (`claude/PROGRESS.md`, `claude/brief.md`,
  `claude/gameplay.md`) are in the claude.ai project; this file is also in
  the zip.
- Python's `pyte` isn't installed by default: `pip install
  --break-system-packages pyte` before playing in the virtual terminal. Use
  `XDG_CONFIG_HOME` (and `HOME`) pointing at a scratch folder so test plays
  don't touch a real save.
- **Building the full game here** (found in M2): proxy.golang.org is
  blocked but GitHub works. Copy the project, then
  `GOPROXY=direct GOSUMDB=off GOFLAGS=-mod=mod go mod tidy`; it fails only on
  `golang.org/x/...` (vanity paths are blocked). Clone those from GitHub at
  old tags that need Go ≤ 1.24: `git clone --depth 1 --branch v0.30.0
  https://github.com/golang/sys`, same for `text` (v0.22.0) and `sync`
  (v0.11.0), add `replace golang.org/x/sys => /path/to/sys` (etc.) to a
  **separate** dev module file, and use `go build/vet/test
  -modfile=dev.mod ./...`. Never ship those replace lines: the real
  `go.mod` stays as it is and Nala's `go mod tidy` fills in the rest.
- For the core alone, a module file with no `require` is enough.
- To play it here: run the built binary in a pseudo-terminal (Python `pty` +
  `pyte`), send keys, read the screen. Bubble Tea waits ~4 s at start for a
  reply the virtual terminal doesn't give; real terminals answer at once.
- Preview frames: `go run ./cmd/snapshot ...` prints ANSI; convert to PNG
  (draw each `▀` cell as two squares) to look at the art. The virtual
  terminal must be 104 × 42.
