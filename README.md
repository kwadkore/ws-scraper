# ws-scraper

`ws-scraper` is a Go-based scraper for Weiss Schwarz data from:

- `https://ws-tcg.com/` (Japanese)
- `https://en.ws-tcg.com/` (English)

It can fetch card data, product listings, expansion lists, and deck rules. The repository includes both a CLI (`wsoffcli`) and an importable Go package (`fetch`) for programmatic use.

## What It Does

The scraper can export:

- card JSON files grouped by language, set, and release
- booster card lists
- expansion lists printed to stdout
- deck rules as a single JSON file

There is also a `products` command for exporting product metadata to `product.json`.

## Go API

The scraper can also be used directly from Go via [`fetch/`](/Users/kweku/Programming/golang/ws-scraper/fetch).

Create a client:

```go
client, err := fetch.NewClient(
	fetch.WithRequestsPerSecond(1),
	fetch.WithBurst(1),
	fetch.WithMaxRetries(4),
)
if err != nil {
	log.Fatal(err)
}
defer client.Close()
```

Main APIs exposed by the `fetch` package:

- `fetch.NewClient(opts ...fetch.Option) (*fetch.Client, error)`
- `(*fetch.Client).Cards(ctx, cfg) ([]fetch.Card, error)`
- `(*fetch.Client).CardsStream(ctx, cfg, ch) error`
- `(*fetch.Client).Boosters(ctx, cfg) (map[string]fetch.Booster, error)`
- `(*fetch.Client).ExpansionList(ctx, cfg) (map[int]string, error)`
- `(*fetch.Client).Products(ctx, page) ([]fetch.ProductInfo, error)`
- `(*fetch.Client).DeckRules(ctx, cfg) (fetch.DeckRules, error)`
- `(*fetch.Client).DeckConstruction(ctx, cfg) ([]fetch.TitleDeckGroup, error)`

Common option helpers:

- `fetch.WithRequestsPerSecond`
- `fetch.WithBurst`
- `fetch.WithNetworkConcurrency`
- `fetch.WithMaxRetries`
- `fetch.WithProxyURL`
- `fetch.WithCache`
- `fetch.WithRespectRobots`
- `fetch.WithRequestTimeout`
- `fetch.WithLogger`

Example: fetch cards directly in Go:

```go
cards, err := client.Cards(context.Background(), fetch.Config{
	Language: fetch.English,
	SetCode:  []string{"IMC"},
})
if err != nil {
	log.Fatal(err)
}

fmt.Println("cards fetched:", len(cards))
```

Example: fetch normalized deck-construction rules:

```go
rules, err := client.DeckRules(context.Background(), fetch.DeckRulesConfig{
	Language: fetch.English,
})
if err != nil {
	log.Fatal(err)
}

fmt.Println("title groups:", len(rules.TitleDeckGroups))
```

## wsoffcli

### Build

Requirements:

- Go 1.21+

Build locally:

```bash
go build -o wsoffcli
```

Run without installing:

```bash
go run . --help
```

Cross-platform builds:

```bash
./build.sh
```

### Docker

Build the image:

```bash
docker build -t wsoffcli .
```

Run it with the current directory mounted as output:

```bash
docker run --rm -v "$PWD:/data" wsoffcli fetch --lang en -n IMC
```

The container writes output under `/data`.

### Quick Start

Fetch all cards whose set code starts with `IMC` from the English site:

```bash
./wsoffcli fetch --lang en -n IMC
```

Fetch multiple set prefixes by separating them with `##`:

```bash
./wsoffcli fetch --lang en -n BD##IM
```

Fetch a specific expansion:

```bash
./wsoffcli fetch --lang ja --expansion 159
```

Fetch recent products only:

```bash
./wsoffcli fetch --lang en --recent
```

Export booster lists instead of individual card files:

```bash
./wsoffcli fetch --lang en --export booster
```

Write deck rules:

```bash
./wsoffcli fetch --lang en --export deckrules
```

Export product metadata:

```bash
./wsoffcli products --page 1
```

### Output Layout

Default output paths:

- cards: `cards/<lang>/<set>/<release>/<set>-<release>-<id>.json`
- boosters: `boosters/<lang>/<release>.json`
- deck rules: `deck-rules_en.json` or `deck-rules_ja.json`
- products: `product.json`

You can override card and booster directories with `--cardDir` and `--boosterDir`.

### Important Flags

Global flags:

- `--expansion`: official expansion number
- `--title`: official title number
- `--neo`, `-n`: set code prefix filter, supports multiple values with `##`
- `--log`, `-l`: log level (`debug`, `info`, `warn`, `error`)
- `--rps`: global request rate limit
- `--burst`: request burst size
- `--net-concurrency`: maximum concurrent network requests
- `--max-retries`: maximum retries per request
- `--proxy-url`: fixed upstream proxy
- `--cache-dir`: enable on-disk HTTP response caching
- `--cache-ttl`: cache lifetime, default `24h`
- `--respect-robots`: honor `robots.txt` and crawl delays

`fetch` flags:

- `--lang`: `en` or `ja`
- `--export`, `-e`: `card`, `booster`, `deckrules`, or `expansionlist`
- `--recent`: fetch recent products
- `--allrarity`, `-a`: include alternate rarities
- `--pagestart`, `-p`: start from a specific page
- `--reverse`, `-r`: reverse page order

### Notes On IDs

- `--expansion` uses the official site expansion ID.
- `--title` uses the official site title ID.
- These values are distinct from each other.
- English and Japanese sites do not share the same numbering.
- Title lookup is only supported where the upstream site exposes it.

See [doc/expansion_list.md](/Users/kweku/Programming/golang/ws-scraper/doc/expansion_list.md) for a snapshot of known expansion IDs.

### Command Docs

Generated command reference lives under [doc/](/Users/kweku/Programming/golang/ws-scraper/doc):

- [doc/wsoffcli_fetch.md](/Users/kweku/Programming/golang/ws-scraper/doc/wsoffcli_fetch.md)
- [doc/wsoffcli_products.md](/Users/kweku/Programming/golang/ws-scraper/doc/wsoffcli_products.md)
- [doc/wsoffcli_completion.md](/Users/kweku/Programming/golang/ws-scraper/doc/wsoffcli_completion.md)

Regenerate them with:

```bash
./wsoffcli gendoc
```

## Development

Run the test suite:

```bash
go test ./...
```

## License

Apache 2.0. See [LICENSE](/Users/kweku/Programming/golang/ws-scraper/LICENSE).
