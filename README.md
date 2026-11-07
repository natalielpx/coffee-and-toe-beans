# Coffee and Toe Beans

A cosy, silent, ambient game for your terminal. You sit at a table in
**Coffee and Toe Beans**, a small cat café on the rooftops of Paris, owned by
Nala. Rain runs down the window, the lamps come on in the evening, the Eiffel
Tower glitters on the hour, and three cats (Brie, Hunter and Mimi) slowly get
to know you. There is nothing to win and nowhere to be.

> **Status:** in development. Milestones 1 to 4 are done: the café and the
> view over Paris, the time of day and the weather, the dialogue box and the
> drinks, the cats, the narrator, and saving. Settings and the first release
> come in milestone 5. See `PROGRESS.md`.

## Play from source

You need [Go](https://go.dev/dl/) 1.24 or newer.

```sh
go mod tidy
go run ./cmd/coffee-and-toe-beans
```

Make your terminal window at least **104 × 42** characters.

## Controls

| Key | What it does |
|-----|--------------|
| ↑ ↓ | choose |
| Enter or Space | confirm; show the whole line at once; turn the page |
| Esc | menu: Order a drink · Look around · Leave the café (Esc again closes it) |
| ← → | in Look around: move between the window, the cats and your table |

The narrator's questions pass by themselves if you leave them alone. A cat
who comes to you waits for your answer, and stays put until you give one.
A drink cools and empties over about 15 minutes; the empty cup stays on the
table until you order another.

## Your save

The café remembers you between visits: how well each cat knows you, the
moments you've shared, your favourite drink, and the time you've spent here.
It saves every minute and when you leave.

| | Where the save lives |
|---|---|
| macOS | `~/Library/Application Support/coffee-and-toe-beans/save.json` |
| Linux | `~/.config/coffee-and-toe-beans/save.json` |

To start afresh, delete that file (or run the game once with `--fresh`,
which keeps the old save as a backup next to it). If the save ever gets
damaged, the game keeps a copy of it, starts afresh and tells you where the
copy is when you leave.

## Previewing (for testing)

```sh
go run ./cmd/coffee-and-toe-beans --time 19:30           # start at any hour
go run ./cmd/coffee-and-toe-beans --fast                 # time runs 30x faster
go run ./cmd/coffee-and-toe-beans --away 3d              # pretend your last visit was 3 days ago (45m, 3h, 2d, 1w)
go run ./cmd/coffee-and-toe-beans --fresh                # start as a first visit (the old save is kept as a backup)
go run ./cmd/coffee-and-toe-beans --weather snow         # clear, cloudy, drizzle, rain, heavy_rain, fog, snow
go run ./cmd/coffee-and-toe-beans --color 256            # force 256 colours (or truecolor)
go run ./cmd/snapshot --time 07:00 --weather fog         # print one still frame and exit
go run ./cmd/snapshot --screen talk --away 3d --page 2   # the welcome back, page 2 (the away note)
go run ./cmd/snapshot --drink chai_latte --sipped 9      # a drink, 9 minutes after it arrived
go run ./cmd/snapshot --screen drinks --time 22:00       # the drinks menu (or: title, menu, talk)
```

Runs with `--fast` or `--time` keep a save of their own (`save-test.json`
next to the real one), so trying things out never changes your real café.

## Changing the art and words

Everything you see or read lives in `content/`:

- `content/sprites/*.txt` — pixel art (see `content/sprites/README.md`)
- `content/interiors/french.json` — the room's layout, colours and lamps
- `content/view.json` — the view of Paris
- `content/lighting.json` — how each hour of the day is lit
- `content/weather.json` — the kinds of weather and how they follow each other
- `content/text/ui.json` — the words of the title card, menus and notes
- `content/dialogue/cafe.json` — the drink offers and what's said about your cup
- `content/narrator/` — the welcome lines, the "while you were away" notes and small talk
- `content/cats/*.json` — each cat: name, poses and habits
- `content/moments/*.json` — each cat's moments, Look around words and away lines
- `content/look.json` — Look around words for the window and your table
- `content/drinks.json` — the drinks menu, serving hours, and how fast a cup is drunk
- `content/pacing.json` — how often things happen

After editing, run `go test ./...`: if something is wrong in a file, it tells
you which file and which line.

## License

MIT — see `LICENSE`.
