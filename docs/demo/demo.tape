# Recorded by record.sh, which drives the tmux session this attaches to.
# vhs cannot press F5 or F3, so every key dv needs is sent by that script.
Output docs/demo/frames/

Set Shell bash
Set FontSize 13
Set Width 960
Set Height 500
Set Padding 12
Set Theme "Catppuccin Mocha"

Hide
Type "tmux attach -t dvdemo"
Enter
Sleep 1s
Show

Sleep 24s
