# Run: python3 scripts/record-search-demos.py
Output docs/demo-search.gif
Set Theme "Catppuccin Mocha"
Set Width 1920
Set Height 1080
Set FontSize 24
Set Framerate 60
Set WindowBar Colorful
Set Padding 20
Set Shell "bash"
Set PlaybackSpeed 1

Env ZLIB_THEME "mocha"
Source docs/demo-search-setup.tape
Source docs/demo-search-actions.tape
