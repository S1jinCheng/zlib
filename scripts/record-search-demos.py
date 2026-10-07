#!/usr/bin/env python3
"""Record both search themes with VHS and an isolated, local Go fixture."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import zipfile

root = Path(__file__).resolve().parent.parent
tapes = ("demo-search.tape", "demo-search-light.tape")
pages = [json.loads((root / "scripts/demos" / name).read_text())
         for name in ("search.json", "search-page2.json")]
if any(len(page["books"]) != 25 for page in pages):
    raise RuntimeError("Search recordings require two pages of 25 book records")


# Theme, output and CLI palette are the only differences between recordings.
def recording_settings(tape):
    return [line for line in (root / "docs" / tape).read_text().splitlines()
            if not line.startswith(("Output ", "Set Theme ", "Env ZLIB_THEME "))]


if recording_settings(tapes[0]) != recording_settings(tapes[1]):
    raise RuntimeError("Search recordings must share dimensions and timing settings")

with tempfile.TemporaryDirectory(prefix="zlib-demo-") as directory:
    workspace = Path(directory)
    executable = workspace / "search-demo"
    fixture = root / "internal/cli/record_demo_test.go"
    with fixture.open("x") as target:
        target.write((root / "scripts/demos/search.go.txt").read_text())
    try:
        subprocess.run(["go", "test", "-c", "-o", str(executable),
                        "./internal/cli"], cwd=root, check=True)
    finally:
        fixture.unlink()
    tests = subprocess.check_output([str(executable), "-test.list=^TestRecordSearchDemo$"], text=True)
    if "TestRecordSearchDemo" not in tests:
        raise RuntimeError("Recording fixture was not compiled")
    launcher = workspace / "zlib"
    launcher.write_text('#!/bin/sh\nexec "' + str(executable) + '" -test.run=^TestRecordSearchDemo$\n')
    launcher.chmod(0o755)
    environment = dict(os.environ, PATH=str(workspace) + os.pathsep + os.environ["PATH"])
    environment.pop("NO_COLOR", None)
    environment["TERM"] = "xterm-256color"
    environment["COLORTERM"] = "truecolor"
    environment["ZLIB_DEMO_FIXTURE"] = str(root / "scripts/demos/search.json")
    filename = "dune.epub"
    epub = workspace / filename
    with zipfile.ZipFile(epub, "w", compression=zipfile.ZIP_STORED) as archive:
        archive.writestr("mimetype", "application/epub+zip")
        archive.writestr("META-INF/container.xml", '<?xml version="1.0"?><container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container"><rootfiles><rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/></rootfiles></container>')
        archive.writestr("EPUB/package.opf", '<?xml version="1.0"?><package version="3.0" unique-identifier="id" xmlns="http://www.idpf.org/2007/opf"><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:identifier id="id">urn:zlib:recording-demo</dc:identifier><dc:title>Zlib recording demo</dc:title><dc:language>en</dc:language><meta property="dcterms:modified">2026-10-07T00:00:00Z</meta></metadata><manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/><item id="chapter" href="chapter.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="chapter"/></spine></package>')
        archive.writestr("EPUB/nav.xhtml", '<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head><body><nav epub:type="toc"><ol><li><a href="chapter.xhtml">Demo</a></li></ol></nav></body></html>')
        archive.writestr("EPUB/chapter.xhtml", '<html xmlns="http://www.w3.org/1999/xhtml"><head><title>Demo</title></head><body><p>This is a local CLI recording sample, not a book download.</p><!--' + " " * 1225000 + '--></body></html>')
    environment["ZLIB_DEMO_EPUB"] = str(epub)
    environment["ZLIB_DEMO_DOWNLOAD_DIR"] = str(Path.home())
    events = workspace / "events.txt"
    environment["ZLIB_DEMO_EVENTS"] = str(events)
    selected = pages[1]["books"][14]
    if selected["extension"] != "epub":
        raise RuntimeError("Original tape must select an EPUB result")
    expected = ["search:1", "search:2", f'selected:{selected["id"]}:{selected["hash"]}',
                "downloaded", f"saved:~/{filename}"]
    target = Path.home() / filename
    # Reserve our own file; never overwrite an existing download in the home dir.
    handle = os.open(target, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    os.close(handle)
    try:
        for tape in tapes:
            events.write_text("")
            subprocess.run(["vhs", str(root / "docs" / tape)], cwd=root, env=environment, check=True)
            observed = events.read_text().splitlines()
            if observed != expected or target.read_bytes() != epub.read_bytes():
                raise RuntimeError(f"{tape}: incomplete EPUB search/download sequence: {observed}")
    finally:
        target.unlink(missing_ok=True)


def frame_timing(path):
    output = subprocess.check_output([
        "ffprobe", "-v", "error", "-select_streams", "v:0", "-show_frames",
        "-show_entries", "frame=best_effort_timestamp_time,pkt_duration_time",
        "-of", "json", str(path),
    ], text=True)
    return json.loads(output)["frames"]


dark = frame_timing(root / "docs/demo-search.gif")
light = frame_timing(root / "docs/demo-search-light.gif")
if dark != light:
    # Browser capture can add a few frames of scheduling jitter despite identical
    # actions. Align the light playback clock to dark after verifying both flows.
    interval = float(dark[1]["best_effort_timestamp_time"])
    dark_duration = float(dark[-1]["best_effort_timestamp_time"]) + interval
    light_duration = float(light[-1]["best_effort_timestamp_time"]) + interval
    if abs(dark_duration - light_duration) > 1:
        raise RuntimeError("Search capture timing differs by more than one second")
    with tempfile.TemporaryDirectory(prefix="zlib-demo-timing-") as directory:
        output = Path(directory) / "light.gif"
        filters = (f"[0:v]setpts=(PTS-STARTPTS)*{dark_duration}/{light_duration},"
                   f"fps={1 / interval},split[a][b];[a]palettegen[p];[b][p]paletteuse")
        subprocess.run(["ffmpeg", "-v", "error", "-y", "-i",
                        str(root / "docs/demo-search-light.gif"), "-filter_complex", filters,
                        "-frames:v", str(len(dark)), "-loop", "0", str(output)], check=True)
        if frame_timing(output) != dark:
            raise RuntimeError("Light playback timing could not be aligned")
        (root / "docs/demo-search-light.gif").write_bytes(output.read_bytes())
print(f"Theme timing matched: {len(dark)} frames")
