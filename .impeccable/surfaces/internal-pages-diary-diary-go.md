---
version: 1
slug: "internal-pages-diary-diary-go"
primary_target: "internal/pages/diary/diary.go"
related_targets: ["internal/pages/settings/settings.go"]
---

# 日记 (Diary)

Scope: the 日记 section of the desktop application — the date tree, the day's
editor, and the 日记配置 section of settings that imports and exports the files.
Mode: Operate. The visitor is the owner, writing a diary entry on their own
machine, often at the end of a day; success is that writing is frictionless and
last night's entry is one click away.

## Direction contract

THESIS: One day is one sheet. The page is a date directory beside a writing
surface, and nothing else competes with the text; it refuses the dashboard habit
of filling the screen with cards, tiles and toolbar buttons.

OWN-WORLD: Cool Graphite unchanged — charcoal shell, white content card, a 44px
band per region with hairline dividers, `theme.Accent` as the only selection
fill, `theme.Muted` for headers and count badges, and the five mood emoji as the
only saturated color. Mono for figures; no other tint earns a place here.

STORY: The reader opens the page and is already in today. Typing is saving. The
tree to the left is the proof of how many days exist; the sheet to the right is
the only thing asking for attention. When a day is done, 已保存 says so quietly
and 删除日记 asks once.

FIRST VIEWPORT: a 56px toolbar (账本 / 今天 / date field, with 已保存 and
删除日记 at the trailing edge) over a 220px date tree (44px header with count,
then year → month → day rows whose numbers and counts line up on two straight
columns) split by a hairline from the editor: a 44px heading band carrying the
date, its weekday, the word count and the one-tab-stop mood picker, then a
full-height writing card.

FORM: two-column directory + sheet, position 1 of the derived structures: the old
application's own shape, migrated rather than re-conceived. seed: none (pinned) —
pinned by the request "迁移日记功能包括 UI 后端" and the project's standing rule
"行为、文案、交互、数据契约一律以旧工程为准，先追平再改进", which fix both the world
(DESIGN.md's Cool Graphite) and this surface's shape (the old diary page's two columns).

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish
review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.
