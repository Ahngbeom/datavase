#!/usr/bin/env bash
# Regenerates docs/demo.gif. Run from the repository root with the test
# database up (make db-up) and ./dv built (make build).
#
# vhs records a tmux session; this script types into that session, since
# vhs itself cannot send the function keys dv runs and copies with.
set -eu

for need in vhs tmux docker; do
	command -v "$need" >/dev/null || { echo "record.sh: $need is required" >&2; exit 1; }
done
[ -x ./dv ] || { echo "record.sh: ./dv is missing; run make build" >&2; exit 1; }

docker exec -i datavase-test-db mariadb -uroot -pdatavase-test < docs/demo/seed.sql

work=$(mktemp -d)
trap 'tmux kill-session -t dvdemo 2>/dev/null || true; rm -rf "$work"' EXIT

tmux kill-session -t dvdemo 2>/dev/null || true
tmux new-session -d -s dvdemo -x 124 -y 32 -c "$work" \
	-e "XDG_CONFIG_HOME=$PWD/docs/demo" \
	-e "XDG_STATE_HOME=$work/state" \
	-e "DATAVASE_PASSWORD_SHOP=datavase-test" \
	-e "PS1=$ " \
	-e "BASH_SILENCE_DEPRECATION_WARNING=1" \
	env "PATH=$PWD:$PATH" bash --noprofile --norc
tmux set-option -t dvdemo status off

vhs docs/demo/demo.tape &
recorder=$!

# say types text the way a person would, so the recording reads as typing.
say() {
	printf '%s' "$1" | while IFS= read -r -n1 ch || [ -n "$ch" ]; do
		case "$ch" in
		" ") tmux send-keys -t dvdemo Space ;;
		";") tmux send-keys -t dvdemo -l "\;" ;;
		*) tmux send-keys -t dvdemo -l "$ch" ;;
		esac
		sleep 0.04
	done
}
key() { tmux send-keys -t dvdemo "$1"; }

sleep 3.5
say "dv open shop"; key Enter
sleep 2
say "SELECT customer, total, status FROM orders ORDER BY total DESC;"
sleep 0.6
key F5
sleep 2.5
key F3
sleep 1
say "c"
sleep 1.2
key Enter
sleep 2.5
key BTab
key Enter; key Enter
say "UPDATE orders SET status = 'refunded';"
sleep 0.6
key F5
sleep 4

wait "$recorder"

# vhs writes the frames and this assembles them: its own encoder produced
# nothing on this machine, silently, and a palette pass keeps the GIF small.
ffmpeg -v error -y -framerate 50 -i docs/demo/frames/frame-text-%05d.png \
	-vf "fps=12,split[a][b];[a]palettegen=max_colors=128[p];[b][p]paletteuse=dither=bayer:bayer_scale=3" \
	docs/demo.gif
rm -rf docs/demo/frames
echo "wrote docs/demo.gif"
