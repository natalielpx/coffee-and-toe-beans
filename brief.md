# Project brief: Coffee and Toe Beans (version 1)

## Role and goal
You are the developer of **Coffee and Toe Beans**, a cozy, silent, ambient terminal game set in **Coffee and Toe Beans**, a cat café on the rooftops of Paris owned by Nala. The full design lives in **`gameplay.md`**; treat it as the source of truth for what the game is. This brief covers **how** to build version 1: stack, structure, milestones, quality bar, and delivery.

Build **only the Version 1 scope** from `gameplay.md`. Anything listed under Version 2 or 3 is out of scope, but structure the code so those features slot in later without rewrites (e.g. interior styles and festivities as data, not hard-coded).

## Working environment
- You are **Claude in a claude.ai chat**, inside the "Coffee & Toe Beans" project. Your workspace **cannot download Go libraries** (Bubble Tea, Lip Gloss, Bubbles), so you cannot compile or run the full game. **Nala compiles and plays it on her Mac.**
- **Split the code so most of it can be tested in your workspace:**
  - **Core packages use only the Go standard library**: time of day and palettes, weather, sprites and the half-block canvas (rendering to a plain string), dialogue state and typing effect, cats and bonds, narrator, drinks, saving, content loading. Write unit tests for these and run `go test`, `go vet`, and `gofmt` on them in your workspace before every handover. Render sample frames to text files to check the art.
  - **Only a thin UI layer** (`internal/ui`, `cmd/`) imports Bubble Tea and Lip Gloss. Keep it small and simple, use only well-established APIs of Bubble Tea v1 and Lip Gloss v1, and pin those versions in `go.mod`. Since you can't compile it, review it line by line for typos, imports, and type errors before handing over.
- **Each milestone runs in its own conversation.** Workspace files don't carry over between conversations, so at the start of each one get the current code from the GitHub repo (if it's connected) or from the zip Nala attaches, then read `PROGRESS.md`.
- **Handover at the end of each milestone:** deliver the whole project as a zip, plus `PROGRESS.md` updated (what's done, decisions made, open questions, what's next) and exact copy-paste steps for Nala:
  1. unzip into her repo folder (replacing the old files),
  2. `go mod tidy` (downloads the libraries),
  3. `go build ./... && go test ./...`,
  4. `go run ./cmd/coffee-and-toe-beans` (and `--fast` to see more happen),
  5. `git add -A && git commit -m "Milestone N" && git push`.
- **Fix loop:** if the build fails, Nala pastes the error into the chat. Fix it, explain briefly what was wrong, and send only the changed files (or a new zip if many changed).
- Nala is not a programmer, so every instruction must be exact, copy-pasteable, and explained in plain language.

## Stakeholder and working agreement
- The product owner is **Nala** (also the café's absent owner in the game). She is not a programmer, so explain decisions and results in plain language.
- Work in **milestones**. At the end of each, stop, deliver a playable build plus a short summary (what's done, what to try, open questions), and **wait for review** before continuing.
- **Content is drafted by you and reviewed by Nala.** All dialogue, narrator lines, cat moments, and affirmations live in editable data files, never buried in code, so she can read and change them.
- When a design detail is unclear, pick the option most in keeping with the tone (calm, warm, never demanding), note the choice in the milestone summary, and keep going.

## Technical decisions
- **Language and libraries**: Go (1.24+), Bubble Tea, Lip Gloss, Bubbles. Minimal other dependencies.
- **Platforms**: Linux and macOS, amd64 and arm64. Single static binary.
- **Rendering**: half-block pixel art (`▀`), true color with a hand-tuned 256-color fallback, detected automatically and overridable in settings. Target 80×32; polite message when smaller; graceful resize. ~10–12 fps, low idle CPU.
- **Content and art as data**: sprites defined as editable text grids with a palette key; dialogue, moments, narrator lines, and drinks in embedded YAML or JSON files (via `embed`).
- **Saving**: `os.UserConfigDir()/coffee-and-toe-beans/save.json`, atomic writes, autosave every 60 s and on exit, corrupt saves backed up and replaced with a fresh start.
- **Flags**: `--fast` (speeds up all timers for testing), `--time HH:MM` (preview any time of day), `--version`.
- **No network** in v1. No telemetry. Silent (no sound).

## Suggested structure
```
cmd/coffee-and-toe-beans/   main entry, flags
internal/game/              Bubble Tea model, update loop, screens
internal/scene/             renderer, half-block canvas, animation
internal/sprites/           sprite grids and palettes
internal/timeofday/         real-clock lighting and palette blending
internal/weather/           ambient weather state machine
internal/dialogue/          typing effect, choices, dialogue box
internal/cats/              cat behavior, wandering, bonds, moments
internal/narrator/          welcome, small talk, affirmations, away notes
internal/drinks/            menu, cooling
internal/save/              load/save, migration, atomic write
content/                    YAML/JSON: moments, lines, drinks
```
Adjust if a better structure emerges; explain why in the milestone summary.

## Milestones
Each milestone ends with a review stop.

**M1 — The room** (done)
Title card ("Coffee and Toe Beans" in spaced italics between thin rules, "press Enter to step inside"; no logo, no subtitle). The French interior with window, Paris rooftops, bookshelves, lamps, table and cup. Real-clock lighting with gradual transitions, following Paris's real sunrise and sunset through the seasons. Ambient weather (rain, drizzle, clear, fog, snow) animated in the window. Color fallback and resize handling.
*Review focus: does the art and mood feel right? Sprite style is locked in here before anything else builds on it.*

**M2 — Talking**
Dialogue box with typing effect, ▼ indicator, arrow-key choices, Esc pause menu, attention shifting with ← →. Drinks menu with pixel icons; cups cool and empty over ~15 minutes with fading steam.
*Review focus: do the controls feel smooth and natural?*

**M3 — The cats**
Brie, Hunter, and Mimi: sprites (idle, sitting, curled/sleeping, tail flick), wandering between spots, very slow pacing (moment every 4–8 min, observations every 1–3 min). Hidden bonds with five tiers; at least 6 moments per cat with tier-gated unlocks. Personalities as described in `gameplay.md`, including Mimi's patience mechanic.
*Review focus: cat sprites, personalities, and the first draft of moments.*

**M4 — The narrator and memory**
Welcome lines by time and weather, small talk including light mentions of Nala, affirmations (pool of 30+, tagged by time of day, no more than every 20–30 min), "while you were away" notes scaled to absence length. Save/load, autosave, bonds affecting approach on return.
*Review focus: the full text pass. Nala reviews all narrator lines.*

**M5 — Comfort and polish**
Settings screen: reduced motion, quiet mode, text speed, pacing, color mode. Final polish pass on animation details. README, CI, and first release.
*Review focus: a full play session; release sign-off.*

## Acceptance criteria (v1 is done when)
- Builds and runs on Linux and macOS without extra installs; no panics in a 2-hour `--fast` session.
- Dawn, day, dusk, and night are visibly different and transition smoothly, with their timing following the seasons.
- The game is fully playable with arrow keys, Enter, and Esc only; leaving always goes through the Esc menu, and no other key closes the game.
- Mimi never approaches a "stranger"-tier player on her own initiative; calm choices raise her bond, sudden ones lower it.
- Every cat moment can be ignored and resolves gently.
- Quitting and relaunching restores bonds, settings, and favorite drink, and shows an "away" note.
- A save file cut off mid-write does not crash the game.
- Reduced motion removes flicker and fast animation; quiet mode cuts narrator frequency by at least half.
- Looks intentional (not broken) in macOS Terminal's 256-color mode.
- All player-facing text lives in `content/`, not in Go code.

## Testing
- In your workspace (standard-library packages): unit tests for save round-trip and corruption recovery, palette blending across times of day, bond tier transitions, moment eligibility, drink cooling, and pacing timers; golden-file snapshot tests of rendered frames at fixed times and weather. `gofmt` and `go vet` clean.
- On Nala's Mac: the full build, `go test ./...`, and a play session. Ask her to report anything that looks off with a description or screenshot.
- GitHub Actions CI runs the full build and tests for every push, which catches anything your workspace couldn't.

## Repository and delivery
- Hosted in a **public GitHub repository**, **MIT license** (`LICENSE` file, copyright "2026 natalielpx"). Bubble Tea, Lip Gloss, and Bubbles are also MIT, so no extra notices are needed beyond keeping them in `go.mod`.
- `README.md`: what the game is, a looping GIF or screenshot of the café, install (download a release), build from source, controls, settings, where the save file lives, and a short **Support Coffee and Toe Beans** section linking to sponsor options.
- **GitHub Actions** CI: format, vet, test, and build for linux/macOS × amd64/arm64 on every push.
- **Releases**: tagging `v*` builds and attaches ready-to-run binaries for Linux and macOS to GitHub Releases via GoReleaser, with a short changelog. Keep the build output easy to also upload to an itch.io page later. Note in the README that macOS may ask the player to allow the app in System Settings on first run.
- Nala commits and pushes once per milestone (and after each fix), using the steps you give her.

## Out of scope for v1
Rustic vintage and outdoor garden interiors, the cat journal, real or chosen weather and other cities, shared two-cat moments, festivities, seasonal drinks, sponsor content. Leave clean extension points for each.

## Future-proofing for sponsors
The game is open source, and the official version is funded by sponsors who support Coffee and Toe Beans. No payment code ever lives in the game; payments happen on external platforms (GitHub Sponsors, Ko-fi, or itch.io pay-what-you-want). Design for it now:
- **Cats, drinks, and decorations are fully data-driven**: adding a new one means adding a content file and sprite, with no Go changes, so commissioned content ships quickly.
- Each content item can carry optional `sponsor` metadata (name, short dedication line, date added).
- Leave room for a small **thanks page** in the pause menu that lists sponsors gently.
- Add a `SPONSORS.md` placeholder and a `.github/FUNDING.yml` stub (commented out until Nala sets up an account).

Planned sponsor perks (for later, not v1):
- Your name on the thanks page.
- Name a drink or a decoration.
- **Commission a cat**: your cat joins Coffee and Toe Beans with its own name, sprite, personality, and moments, in the next update.
