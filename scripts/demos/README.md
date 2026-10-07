# Search recordings

Run `python3 scripts/record-search-demos.py` from the repository root. Requires Go, VHS, ttyd, and ffmpeg.

Both themes replay the original `docs/demo-search.tape` actions: 25 results, three moves down, next page, fourteen moves down, select/download, then a ten-second pause. Input timing, 700 ms browsing pauses, three-second response pauses, playback speed, and recording settings match the original. `docs/demo-search-actions.tape` contains the unchanged action sequence; hidden shell setup is separate.

Public metadata for two pages was captured on 2026-10-07 from `POST https://z-lib.gl/eapi/book/search` (`message=dune`, `page=1/2`, `limit=25`). Only book metadata is retained; download links and access tokens are excluded. Two adjacent rows on page two are swapped so the original selection lands on a real EPUB edition.

The fixture uses the original single-input prompt and the actual search and download models. Download progress transfers a generated EPUB sample from a local server into the user home directory, displayed as `~/dune.epub`. The script reserves that filename without overwriting existing files and removes its sample after recording. No saved session or real book file is used. The script checks both page requests, EPUB selection, downloaded bytes, the abbreviated saved path, matching settings, and GIF frame timestamps.

If browser capture adds a few frames of timing drift, the light GIF playback clock is resampled to match dark after both complete flows are verified. The original action tape stays unchanged.
