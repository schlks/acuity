import io
import re
import string
import sys
from pathlib import Path

import uharfbuzz as hb
from fontTools import subset
from fontTools.ttLib import TTFont

UI = Path(__file__).resolve().parent.parent
SRC_DIR = UI / "src"
FULL_FONT = UI / "fonts-src" / "material-symbols-outlined.full.woff2"
OUT_FONT = UI / "static" / "fonts" / "material-symbols-outlined.woff2"

TAG_BODY = r"(?:[^>{]|\{[^{}]*\})*?"
ICON_ELEMENT = re.compile(
    rf"<(\w+)\s{TAG_BODY}\bmaterial-symbols-outlined\b{TAG_BODY}>(.*?)</\1\s*>",
    re.DOTALL,
)
STATIC_NAME = re.compile(r"^\s*([a-z0-9_]+)\s*$")
QUOTED_NAME = re.compile(r"""['"`]([a-z0-9_]+)['"`]""")
EXPRESSION = re.compile(r"\{[^{}]*\}")
BARE_NAME = re.compile(r"(?<![\w.])([a-z][a-z0-9_]*)(?![\w.])")
LIGATURE_CHARS = string.ascii_lowercase + string.digits + "_"


def collect_icon_names():
    required, optional = {}, {}
    for path in sorted(SRC_DIR.rglob("*")):
        if path.suffix not in {".svelte", ".html", ".ts", ".js"}:
            continue
        text = path.read_text(encoding="utf-8")
        for match in ICON_ELEMENT.finditer(text):
            content = match.group(2)
            line = text.count("\n", 0, match.start()) + 1
            where = f"{path.relative_to(UI)}:{line}"
            static = STATIC_NAME.match(content)
            if static:
                required.setdefault(static.group(1), where)
                continue
            expressions = EXPRESSION.findall(content)
            bare = EXPRESSION.sub(" ", content)
            for name in BARE_NAME.findall(bare):
                required.setdefault(name, where)
            for expression in expressions:
                for name in QUOTED_NAME.findall(expression):
                    optional.setdefault(name, where)
    return required, optional


def resolve_glyphs(font_path, names):
    tt = TTFont(font_path)
    tt.flavor = None
    raw = io.BytesIO()
    tt.save(raw)
    font = hb.Font(hb.Face(hb.Blob(raw.getvalue())))
    glyph_order = tt.getGlyphOrder()
    cmap = tt.getBestCmap()
    letters = {cmap[ord(c)] for c in LIGATURE_CHARS if ord(c) in cmap}

    resolved, unknown = {}, []
    for name in names:
        buf = hb.Buffer()
        buf.add_str(name)
        buf.guess_segment_properties()
        hb.shape(font, buf)
        gids = [info.codepoint for info in buf.glyph_infos]
        if len(gids) == 1 and gids[0] != 0 and glyph_order[gids[0]] not in letters:
            resolved[name] = glyph_order[gids[0]]
        else:
            unknown.append(name)
    return resolved, unknown


def main():
    required, optional = collect_icon_names()

    resolved, unknown = resolve_glyphs(FULL_FONT, required)
    if unknown:
        for name in sorted(unknown):
            print(f"error: unknown icon '{name}' ({required[name]})", file=sys.stderr)
        sys.exit(1)

    candidates = {n: w for n, w in optional.items() if n not in resolved}
    extra, ignored = resolve_glyphs(FULL_FONT, candidates)
    resolved.update(extra)

    font = TTFont(FULL_FONT)
    glyph_order = set(font.getGlyphOrder())
    glyphs = set(resolved.values())
    glyphs |= {f"{g}.fill" for g in glyphs if f"{g}.fill" in glyph_order}

    options = subset.Options()
    options.flavor = "woff2"
    options.layout_features = ["rlig", "rclt"]
    options.layout_closure = False
    options.notdef_outline = True

    subsetter = subset.Subsetter(options)
    subsetter.populate(glyphs=sorted(glyphs), text=LIGATURE_CHARS)
    subsetter.subset(font)
    font.flavor = "woff2"
    generated = io.BytesIO()
    font.save(generated)

    if "--check" in sys.argv[1:]:
        if not OUT_FONT.exists() or OUT_FONT.read_bytes() != generated.getvalue():
            print(f"error: ui/{OUT_FONT.relative_to(UI)} is out of date, run 'make icons'", file=sys.stderr)
            sys.exit(1)
        print(f"ui/{OUT_FONT.relative_to(UI)} is up to date ({len(resolved)} icons)")
        return

    OUT_FONT.write_bytes(generated.getvalue())

    if ignored:
        print("ignored non-icon literals: " + ", ".join(f"{n} ({candidates[n]})" for n in sorted(ignored)))
    aliases = sorted(n for n, g in resolved.items() if n != g)
    print(f"{len(resolved)} icon names -> {len(set(resolved.values()))} glyphs (+{len(glyphs) - len(set(resolved.values()))} fill variants)")
    if aliases:
        print("aliases: " + ", ".join(f"{n} -> {resolved[n]}" for n in aliases))
    print(f"wrote ui/{OUT_FONT.relative_to(UI)} ({OUT_FONT.stat().st_size // 1024} KiB)")


if __name__ == "__main__":
    main()
