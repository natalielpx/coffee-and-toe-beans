# Coffee and Toe Beans — gameplay design

A cozy, ambient terminal game. You sit at a table in **Coffee and Toe Beans**, a small cat café on the rooftops of Paris owned by Nala. You order drinks, watch the weather, and slowly befriend three café cats through gentle, visual-novel-style choices. It's meant to be left open for long stretches: slow, warm, and never demanding.

## Vision and tone
- Calm, warm, a little funny. Never stressful.
- No fail states, no punishing timers, no numbers going up.
- The scene is the star; text stays small and soft.
- Silent by design. Players bring their own music.
- Doing nothing is a valid way to play.

## The café: Coffee and Toe Beans
- Owned by **Nala**, who is never there in person. Her presence is felt through the place itself: her choice of books, the cushions she picked, the way the cats wait by the door at closing time.
- The narrator mentions her lightly and warmly from time to time ("Nala's handwriting is still on the menu chalkboard"). She is never described in detail.
- The player has no name, backstory, or specifics. They are simply a guest.

### Interior styles (chosen in settings)
Every style keeps the **window** and **bookshelves**.
1. **Rustic vintage**: exposed wood beams, worn leather armchair, old record player, mismatched mugs, warm brown tones.
2. **French**: Haussmann moldings, marble bistro tables, cane chairs, brass lamps, posters in faded frames. Palette "warm clay, charcoal wood": sand walls, peach trim, charcoal shelves and panels, teal curtains, gold brass.
3. **Outdoor garden**: a covered rooftop terrace with plants, string lights, a glass canopy the rain runs down, potted lavender, a wrought-iron table.

## The scene
- Pixel art drawn with half-block characters (two pixels per cell), true color with a 256-color fallback.
- A view from your table: the window onto Paris rooftops (zinc roofs, chimney pots, distant lit windows, the Eiffel Tower tip on clear nights, sparkling on the hour in the evening), bookshelves, lamps, plants, your cup, and the cats.
- Subtle animation: rain on the glass, steam from the cup, lamp flicker, tail flicks, ear twitches.
- Calm surfaces (plain walls, faint marble, even floor); detail lives in the bookshelf and the rooftops.

## Title card
Just the words "Coffee and Toe Beans", letter-spaced in italics between two long thin rules, over the dimmed café, then "press Enter to step inside". No logo, no subtitle.

## Time of day (real clock)
- Lighting and sky shift gradually: dawn, morning, afternoon, golden evening, deep navy night with amber lamps.
- Follows the seasons: real Paris sunrise and sunset for each date, so winter evenings are dark early and summer evenings stay light.
- Lamps turn on in the evening.
- Narration and cat behavior follow the hour: sleepy at night, playful in the evening.

## Weather
Three modes in settings:
1. **Real weather** for a chosen city (default Paris), via Open-Meteo. Option to use your clock or the city's clock.
2. **Weather of choice**: rain, drizzle, heavy rain, clear, cloudy, snow, fog.
3. **Ambient** (offline default): mostly rain, with variety and rare winter snow.

## Controls
Arrow keys only, like Japanese visual novels.
- **↑ ↓** choose · **Enter / Space** confirm, skip typing, advance
- **← →** shift attention between the window, the cat corner, and your table
- **Esc** pause menu: leave the café, order a drink, cat journal, settings (order to be confirmed in M2; "Leave the café" is currently first)

## Dialogue
- Visual-novel dialogue box with letter-by-letter typing and a blinking ▼.
- 2–4 choices per moment. Ignoring a moment lets it pass gently.

## The cats
- **Brie**: fluffy cream cat with a Parisian personality. Elegant, a little haughty, theatrical sighs. Likes calm guests and café crème.
- **Hunter**: playful orange-brown tabby. Chases steam, strings, and raindrops. Comes often, stays briefly, most active in the evening.
- **Mimi**: shy Siamese with blue eyes. Rewards patience and quiet choices; sudden actions make her retreat. Her visits feel special.

### Friendship
- Hidden bond per cat, never shown as a number.
- Tiers: stranger → curious → familiar → friend → "your cat".
- Higher tiers unlock new moments (Brie on your table, Hunter bringing a toy, Mimi asleep on your lap) and make cats approach sooner when you arrive.

### Pacing (very slow)
- A cat moment roughly every 4–8 minutes.
- Small ambient observations every 1–3 minutes.
- Randomized so it never feels mechanical.

## Drinks
Café crème, chocolat chaud, chai latte, thé Earl Grey, matcha latte, vin chaud (evenings), tisane verveine (late night). Ordered from a shop-style menu with pixel icons. A drink cools and empties over about 15 minutes. Seasonal drinks appear during festivities.

## The narrator
A soft, warm, second-person voice that:
- Welcomes you, tuned to time and weather.
- Makes small talk about the café, the city, and Nala's touches.
- Offers occasional affirmations (no more than every 20–30 minutes, never preachy).
- On return, shares what you missed while away, built from the cats' personalities.

## Seasons and festivities
Based on the real date. Each adds decorations to the scene, a seasonal drink or treat, special narrator lines, and sometimes a unique cat moment. Lunar-calendar dates need a lookup table (or a small lunar calendar library) covering at least 2026–2035.

**Chinese**
- Lunar New Year: red lanterns, paper cuttings, oranges on the table; a cat curled near a red envelope.
- Lantern Festival: floating lanterns outside, tangyuan.
- Qingming: quiet, reflective lines, fresh spring greenery.
- Dragon Boat Festival: zongzi on the counter.
- Qixi: two stars brighter in the night sky.
- Mid-Autumn Festival: full moon over the rooftops, mooncakes, lanterns; cats watching the moon.
- Dongzhi (winter solstice): tangyuan, the longest night, extra-warm narration.

**Malaysian**
- Hari Raya Aidilfitri: ketupat decorations, pelita oil lamps, kuih on the table.
- Deepavali: oil lamps (diya), kolam pattern at the door, sweets.
- Thaipusam: respectful, gentle narrator mention.
- Merdeka (31 Aug) and Malaysia Day (16 Sep): small flag bunting, teh tarik as a seasonal drink.
- Christmas in Malaysia and Chinese New Year open-house spirit: "the café feels like everyone's welcome tonight."

**Others**
- Christmas: lights on the rooftops, vin chaud, a cat in a box.
- New Year's Eve: distant fireworks over Paris at midnight.
- Halloween: a tiny pumpkin; Hunter suspicious of it.
- Bastille Day (14 July): fireworks seen from the window.
- First snow of the year: a special narrator moment and cat reactions.

## Comfort settings
- **Reduced motion**: slower, fewer animations; no flicker; rain becomes calm and sparse.
- **Quiet mode**: fewer narrator lines and affirmations; cats still come and go.
- Also: text speed, pacing (very slow / gentle), color mode, interior style, weather mode and city.

## Cat journal
One page per cat: pixel portrait, a description that grows with the bond, and a list of memories ("Mimi slept on your lap during the first snow."). Unmet cats appear as silhouettes.

## Saving
- Saved in the user's config folder (`coffee-and-toe-beans/save.json`).
- Persists bonds, unlocked moments, journal memories, last visit, favorite drink, time spent, and all settings.
- Autosaves every 60 seconds and on exit; atomic writes; corrupt saves are backed up and the game starts fresh.

## Platform
Go + Bubble Tea + Lip Gloss. Linux and macOS, single binary. Target window 104×42 (a 104 × 84-pixel scene) with a polite message if smaller. Hidden `--fast` flag for testing.

---

## Scope

### Version 1 (must ship polished)
1. Title card, the scene, and the **French** interior style.
2. Real-clock time of day with gradual lighting, following the seasons.
3. Ambient weather mode (rain, drizzle, clear, fog, snow).
4. Dialogue engine with arrow-key choices and typing effect.
5. Brie, Hunter, and Mimi: sprites, wandering, hidden bonds, at least 6 moments each.
6. Drinks menu with cooling cups.
7. Narrator: welcome lines, small talk, affirmations, and "while you were away" notes.
8. Save/load and autosave.
9. Comfort settings: reduced motion and quiet mode.
10. True color with 256-color fallback; graceful resizing.

### Version 2
- Rustic vintage and outdoor garden interior styles.
- Cat journal with memories and portraits.
- Real weather via Open-Meteo, weather of choice, other cities.
- More moments per cat (8–10 each) and shared two-cat moments.

### Version 3
- Seasons and festivities, starting with Lunar New Year, Mid-Autumn, Hari Raya, Deepavali, and Christmas, then the rest.
- Seasonal drinks and decorations.
- Extra polish: more ambient details, more narrator lines.

### Later: sponsors
- Sponsors can pay to add something to Coffee and Toe Beans: a new cat (with its own name, personality, and moments), a drink, or a decoration.
- Each sponsored item can carry a small dedication, shown in a gentle credits / thanks page.
- Sponsored content should feel like part of the café, never like an ad: same art style, same tone, no pop-ups or banners.
- The game is open source (MIT). Sponsors pay for their cat or item to live in the official Coffee and Toe Beans, via external platforms like GitHub Sponsors, Ko-fi, or itch.io. Payments never happen inside the game.
- Perks: a thank-you on the credits page, naming a drink or decoration, or commissioning a cat.
