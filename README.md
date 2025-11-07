# go-mfire

A small, focused CLI that demonstrates how to parse [MangaFire](https://mangafire.to)
and run searches that require the site's `vrf` token. This repository is meant
for learning and experimentation — not for large-scale scraping.

> [!IMPORTANT]
> Educational purpose only — this project is provided under the MIT license.
> It is not intended to help circumvent paywalls, license restrictions, or
> facilitate piracy. Use at your own risk and only with data you own or have
> permission to process.

<p align="center">
	<img src="https://github.com/galpt/go-mfire/screenshot/how-it-looks-like.jpg" alt="Interface preview" style="max-width:100%;height:auto;" />
	<br/>
	<em>How it looks like</em>
</p>

## Table of contents

- [Status](#status)
- [What it does](#what-it-does)
- [Quick start (Windows PowerShell)](#quick-start-windows-powershell)
- [Usage notes](#usage-notes)
- [Developer notes](#developer-notes)
- [Configuration — VRF cache](#configuration--vrf-cache)
- [License & intent](#license--intent)
- [Contributing](#contributing)

## Status

- Minimal CLI implemented.
- Home-page listing and search support (limited to 10 results).
- VRF token generation ported from kotatsu/mihon and wired into search.

## What it does

- Fetches and shows up to 10 manga titles from the MangaFire home page.
- Provides an interactive CLI to view a title's URL or run a text search.
- Implements MangaFire's `vrf` token generator so searches work reliably.

## Quick start (Windows PowerShell)

1. Build the binary from this folder:

```powershell
.\compile.bat
```

2. Run it:

```powershell
.\mfire.exe
```

> [!TIP]
> On Windows you can use the included `compile.bat` for a quick iterative
> build loop — it runs `gofmt` (if configured) and `go build` so you can
> recompile fast while developing.

## Usage notes

- On start the CLI prints up to 10 titles found on the home page.
- Type a number to print the selected title's URL.
- Type `search` and enter a query to fetch up to 10 search results.
- Type `exit` to quit.

## Developer notes

- Module: `github.com/galpt/go-mfire`
- Library: `pkg/mfire` contains the parser, VRF generator and public helpers.

## Configuration — VRF cache

The VRF generator is moderately expensive to compute, so the package keeps an
in-memory LRU cache to avoid recomputing tokens for repeated queries. You can
configure the cache size in two ways:

### 1) Environment variable (recommended)

Set `MGFIRE_VRF_CACHE_SIZE` to a positive integer before launching the CLI.
Example (PowerShell):

```powershell
$env:MGFIRE_VRF_CACHE_SIZE = "2048"; .\mfire.exe
```

### 2) Programmatically

If you use `pkg/mfire` from Go, call `mfire.SetVrfCacheSize(n)` early in your
program. Use `mfire.GetVrfCacheSize()` to inspect the current capacity.

> [!NOTE]
> Default cache size: `1024` entries.
>
> Passing a non-positive value to `SetVrfCacheSize` is a no-op. The
> package-level cache is safe for concurrent use.

## License & intent

MIT — see the `LICENSE` file. This project is for research and learning. Please
respect site terms and avoid abusive scraping.

## Contributing

Contributions welcome. Open an issue or send a pull request for bugs, tests,
or enhancements. If you'd like a small library-style API instead of the CLI,
say so and I can add example usage.
