#!/usr/bin/env bash
# Refresh the embedded copy of the HOP asset manifest and the vendored home engine from hel.ink, then rebuild.
# The asset version is read from the manifest ("version"), so a new hop-assets release needs no edit here.
#   scripts/sync-hop-assets.sh              # fetch from https://hel.ink
#   scripts/sync-hop-assets.sh /path/HEL.ink  # copy from a HEL.ink checkout
set -euo pipefail
cd "$(dirname "$0")/.."
src="${1:-}"
fetch() { # fetch <path under public/hop-assets> <destination>
  mkdir -p "$(dirname "$2")"
  if [[ -n "$src" ]]; then cp "$src/public/hop-assets/$1" "$2"; else curl -fsS "https://hel.ink/hop-assets/$1" -o "$2"; fi
}
fetch apps.json web/hopassets/apps.json.new
python3 -c 'import json,sys; json.load(open(sys.argv[1]))' web/hopassets/apps.json.new
mv web/hopassets/apps.json.new web/hopassets/apps.json
ver=$(python3 -c 'import json; print(json.load(open("web/hopassets/apps.json"))["version"])')
[[ "$ver" =~ ^v[0-9]+$ ]] || { echo "manifest version '$ver' is not vN" >&2; exit 1; }
slugs=$(python3 -c 'import json; print(" ".join(a["slug"] for a in json.load(open("web/hopassets/apps.json"))["apps"] if a.get("public")))')
for file in hop-bento.css hop-home.css hop-home.js hop-hero3d.js hop-motion.js hop-scenes.js hop-scenes.css; do
  fetch "$ver/$file" "web/static/hop-home/$file"
done
for slug in $slugs; do # the family tiles show every public app's picture
  for look in light dark; do fetch "$ver/home/$slug-$look.webp" "web/static/hop-home/home/$slug-$look.webp"; done
done
echo "hop-assets $ver"
git --no-pager diff --stat -- web/hopassets/apps.json
