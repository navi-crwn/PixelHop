#!/usr/bin/env python3
"""Builds the project website (GitHub Pages, served from docs/) in every language.

site/template.html is the English page; every piece of visible text is wrapped
in [[...]]. Each site/strings/<code>.json maps that English text to its
translation. Run `python3 site/build.py` after editing either; it writes
docs/index.html (English) and docs/<code>/index.html, and fails if a
translation is missing so no page ships half-translated.
"""
import html
import json
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent
DOCS = ROOT.parent / "docs"
BASE = "https://navi-crwn.github.io/pixelhop/"

# English first (the main language), the rest by their English names.
LANGS = [
    ("en", "en", "EN", "English"),
    ("zh-cn", "zh-CN", "中文", "简体中文"),
    ("de", "de", "DE", "Deutsch"),
    ("id", "id", "ID", "Bahasa Indonesia"),
    ("ja", "ja", "日本語", "日本語"),
    ("pt-br", "pt-BR", "PT", "Português (Brasil)"),
    ("es", "es", "ES", "Español"),
]

MARK = re.compile(r"\[\[(.*?)\]\]", re.S)


def url(code):
    return BASE if code == "en" else BASE + code + "/"


def lang_menu(current, root):
    items = []
    for code, tag, _, name in LANGS:
        href = root + ("" if code == "en" else code + "/")
        cur = ' aria-current="page"' if code == current else ""
        items.append(f'<a href="{href or "./"}" hreflang="{tag}" lang="{tag}"{cur}>{name}</a>')
    short = next(s for c, _, s, _ in LANGS if c == current)
    return ('<details class="lang-menu"><summary aria-label="[[Language]]"><svg><use href="#i-globe"/></svg>'
            f'<span>{short}</span></summary><div class="lang-list">' + "".join(items) + "</div></details>")


def build():
    tpl = (ROOT / "template.html").read_text()
    alternates = "\n".join(
        f'<link rel="alternate" hreflang="{tag}" href="{url(code)}">' for code, tag, _, _ in LANGS
    ) + f'\n<link rel="alternate" hreflang="x-default" href="{BASE}">'
    problems = []
    for code, tag, _, _ in LANGS:
        root = "" if code == "en" else "../"
        page = (tpl.replace("{{langmenu}}", lang_menu(code, root))
                   .replace("{{alternates}}", alternates)
                   .replace("{{url}}", url(code))
                   .replace("{{lang}}", tag)
                   .replace("{{root}}", root))
        strings = {}
        if code != "en":
            strings = json.loads((ROOT / "strings" / f"{code}.json").read_text())

        def tr(m):
            en = m.group(1)
            if code == "en":
                return en
            t = strings.get(en)
            if not t:
                problems.append(f"{code}: missing {en[:70]!r}")
                return en
            if t.count('"') != en.count('"'):  # markup may carry quotes; plain text must not (it can sit in an attribute)
                problems.append(f"{code}: use typographic quotes, not \" in {t[:70]!r}")
            return t

        page = MARK.sub(tr, page)
        if code != "en":
            unused = set(strings) - set(MARK.findall(tpl)) - {"Language"}
            for u in sorted(unused):
                problems.append(f"{code}: unused string {u[:70]!r}")
        out = DOCS / ("index.html" if code == "en" else f"{code}/index.html")
        out.parent.mkdir(parents=True, exist_ok=True)
        out.write_text(page)
    if problems:
        print("\n".join(problems), file=sys.stderr)
        sys.exit(1)
    print(f"built {len(LANGS)} pages")


if __name__ == "__main__":
    build()
