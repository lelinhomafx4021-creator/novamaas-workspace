#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
mkdir -p src/assets/tabbar

draw_icon() {
  local name="$1"
  local drawing="$2"
  local color output
  for state in inactive selected; do
    if [[ "$state" == selected ]]; then
      color='#4f46e5'
      output="src/assets/tabbar/${name}-selected.png"
    else
      color='#8a94a8'
      output="src/assets/tabbar/${name}.png"
    fi
    magick -size 64x64 xc:none -fill none -stroke "$color" -strokewidth 4 \
      -draw "$drawing" "$output"
    magick "$output" -resize 56x56 -gravity center -background none -extent 64x64 "$output"
  done
}

draw_icon home "path 'M 9,28 L 32,10 L 55,28 L 55,53 Q 55,56 52,56 L 12,56 Q 9,56 9,53 Z' path 'M 25,56 L 25,37 L 39,37 L 39,56'"
draw_icon models 'roundrectangle 9,9 27,27 4,4 roundrectangle 37,9 55,27 4,4 roundrectangle 9,37 27,55 4,4 roundrectangle 37,37 55,55 4,4'
draw_icon chat "roundrectangle 10,9 54,49 8,8 polyline 26,49 32,56 38,49 line 21,26 43,26 line 21,35 36,35"
draw_icon usage 'roundrectangle 10,10 54,54 4,4 line 20,44 20,32 line 32,44 32,21 line 44,44 44,28'
draw_icon profile "circle 32,23 43,23 path 'M 12,54 C 14,44 22,38 32,38 C 42,38 50,44 52,54'"
