# go-sheet

```
type Sample struct {
  Name  string   `sheet:"name"`
  Num   int      `sheet:"num"`
  Term  Term     `sheet:"term"`
  IDs   []int    `sheet:"ids,csv"`
  Array []string `sheet:"array`
}

type Term struct {
  Start int64 `sheet:"start,datetime"`
  End   int64 `sheet:"end,datetime"`
}
```

[{name, a}, {num, 9}, {term.start: 9}, {term.end:8}]

