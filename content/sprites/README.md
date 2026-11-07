# Pixel art files

All of the café's hand-drawn pixel art lives in the `.txt` files in this
folder. Each picture (a "sprite") is a little grid of letters, and a list
above it says which colour each letter means. You can open these files in any
text editor, change them, rebuild the game, and see the result.

## A small example

```
palette brass
  b #b48a3c  brass
  B #e0bd6c  brass highlight

sprite table_lamp
  use brass
  s #f0e0c0 @lamp  pleated shade
grid
..sss..
.sssss.
sssssss
...B...
...b...
..bBb..
end
```

- `palette NAME` starts a list of colours that several sprites can share.
- `sprite NAME` starts a picture. Its name is how the interior files
  (`content/interiors/*.json`) refer to it.
- `use NAME` borrows the colours of a palette.
- Each colour line is: **one character**, then a **colour** like `#e0bd6c`
  (or `none` for see-through), then optionally a **behavior** starting with
  `@`, then any note you like.
- `grid` starts the picture, `end` finishes it. In the grid, `.` and spaces
  are see-through. Rows can be different lengths; short rows are padded.
- Lines starting with `#` are comments.

Each character is one pixel. On screen, two pixels stack into one terminal
cell, so pixels are square.

## Behaviors

| Word      | What it does |
|-----------|--------------|
| `@lamp`   | Glows warm when the lamps are on (lampshades). The sprite must be given a `"light"` in the interior file. |
| `@flame`  | A candle flame: always flickers, unlit in daytime. Also needs a `"light"`. |
| `@roof`   | Snow settles here. |
| `@window` | A distant window that may light up at night. |
| `@eiffel` | Part of the Eiffel Tower: golden when its lights are on, sparkles on the hour. |
| `@beacon` | The tower's searchlight at the very top. |
| `@drink`  | The inside of a cup or glass. It's filled with the drink's colours (from `content/drinks.json`) and empties from the top as you sip; the colour you write is what the empty cup shows. |

## Colours

Draw everything as it looks in **full daylight**. The game dims, warms and
cools it for the time of day and the weather, and adds lamp light at night.

## If something goes wrong

If a file has a mistake, the game (and `go test ./...`) stops with a message
naming the file and the line, for example:

```
sprites/french.txt line 42: grid row 3 uses "x", which isn't in its palette
```
