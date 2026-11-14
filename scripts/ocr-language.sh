#!/bin/sh
# Download reading languages for the studio's "Image to text" into $PIXELHOP_DATA/tessdata.
# usage: scripts/ocr-language.sh deu fra jpn      (Tesseract language codes; English is already built in)
# Source: the official tessdata models (4.0.0_best_int) published by the Tesseract.js project.
set -eu
dir="${PIXELHOP_DATA:-./data}/tessdata"
mkdir -p "$dir"
[ "$#" -gt 0 ] || { echo "usage: $0 LANG [LANG...]   e.g. $0 deu fra jpn" >&2; exit 2; }
for l in "$@"; do
  case "$l" in *[!a-z_]*) echo "bad language code: $l" >&2; exit 2;; esac
  echo "Downloading $l ..."
  curl -fL --proto '=https' -o "$dir/$l.traineddata.gz.part" "https://tessdata.projectnaptha.com/4.0.0_best_int/$l.traineddata.gz"
  mv "$dir/$l.traineddata.gz.part" "$dir/$l.traineddata.gz"
done
echo "Done. Reload the studio: it now offers: $(ls "$dir" | sed 's/\.traineddata\.gz//' | tr '\n' ' ')"
