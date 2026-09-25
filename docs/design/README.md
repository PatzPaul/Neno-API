# Handoff: Neno — Swahili-first Bible & SDA learning app

## Overview
Neno is a mobile app (iOS + Android, built with **Expo / React Native**) that combines a TikTok-style vertical content feed with a full learning library for Seventh-day Adventist members, youth, elders and seekers. Swahili is the primary language; English and French are secondary/parallel languages.

Feature areas:
- **Mlisho (Feed)** — vertical, one-item-per-swipe feed: Bible verses, Ellen G. White quotes, Sabbath School prompts, prophecy videos, hymns, health tips, youth/Pathfinder challenges.
- **Biblia** — full Bible reader with parallel translation and chapter audio.
- **Maktaba (Library)** — EGW books, the 28 Fundamental Beliefs, Bible study courses with quizzes, Sabbath School quarterly.
- **Nyimbo (Hymnal)** — Nyimbo za Kristo + SDA Hymnal with lyrics and audio.
- **Mimi (Me)** — profile, saved items, notes & highlights, settings, Sabbath sunset times.

Companion docs in this folder:
- `BACKEND.md` — Go + PostgreSQL architecture, API surface, content pipeline.
- `CLAUDE.md` — drop at the root of the new monorepo; standing instructions for Claude Code.
- `db/001_init.sql` — initial Postgres schema.
- `mobile/theme.ts` — design tokens ported to React Native.

## About the Design Files
The files in `design/` are **design references created in HTML** — prototypes showing intended look and behavior, **not production code to copy**. Recreate them in Expo / React Native using idiomatic patterns (Expo Router, `FlatList` with paging, `StyleSheet`/theme tokens). Open `design/Neno App.dc.html` in a browser to view every screen on one pan/zoom canvas; the feeds actually scroll and the quiz/toggles are clickable.

## Fidelity
**High-fidelity** for layout, color, type and spacing. Recreate pixel-close at a 360×760 dp reference frame. Copy is **sample content**: Swahili Bible text is SUV as recalled (verify against the licensed USFM), EGW Swahili lines are drafts, hymn numbers/titles are placeholders. All real content comes from the backend.

**Chosen feed direction:** the canvas shows 3 feed directions (1a Blueprint cards, 1b Steel field, 1c Study stack). All other screens use **1a**. Implement 1a as the default feed; 1b is a strong candidate for a "Video" feed tab; 1c's "Soma pamoja" link list should appear as a bottom sheet from any feed item. Confirm with product owner if unsure.

## Global layout
- Reference frame 360×760. Status bar 28. Bottom tab bar 60 high, 5 equal columns, 1px top border `divider`.
- Tabs (icon 20 + label 11px, gap 3): **Mlisho** (home), **Biblia** (book-open), **Maktaba** (library), **Nyimbo** (music), **Mimi** (user). Active = `accent700`, inactive = `neutral600`; on dark screens active = `bg`, inactive = `accent400`.
- Screen horizontal padding 18 (feed 20).
- Every touch target ≥ 44×44 (feed rail 48×48).

## Visual language ("Industry" blueprint system)
- Square corners everywhere (radius 0). Hairline 1px borders in `divider`.
- **Blueprint frame**: cards, figures and primary buttons carry four "+" registration marks at the corners — 11×11 crosshair, 1px lines, color `text` @55%, offset −6px outside each corner. Build once as `<Blueprint>` wrapper component.
- Cards are transparent (no fill). The **primary button** is the only solid object: `accent` fill, `bg` text, square, with corner marks.
- Imagery: duotone washed into `accent` (color blend). Placeholders in the mock are diagonal stripes `neutral300` 1px / 9px gap with a monospace label.
- Icons: **Lucide** (`lucide-react-native`), strokeWidth **1.5**, size 22 (rail/headers), 20 (tabs), 14 (inline).
- Section labels: 11px, letter-spacing 0.1em, uppercase, `accent700` or `neutral600`.
- Reference labels (e.g. "YOHANA 3:16"): Barlow Condensed 15–16px, letter-spacing 0.08em, uppercase, `accent700`.
- Segmented control: 1px `divider` outline, options separated by 1px lines, selected = `accent` fill + `bg` text, 13px, min-height 36.
- Chips: 13px, padding 6×12, 1px border; selected = `accent` fill.
- Tags: 11px, padding 3×10; `tag-accent` = bg `accent100` / text `accent800`; `tag-neutral` = `neutral100` / `neutral800`; `tag-outline` = 1px `accent` border + `accent` text.

## Screens / Views

### 1a — Feed (Mlisho), default
- **Purpose**: swipe through mixed daily content.
- **Layout**: full-height pager (`FlatList` `pagingEnabled`, item height = screen − status − tabs = 672). Header overlaid at top (bg @92%): "Kwa Ajili Yako" (Barlow Condensed 600, 17px, 2px `accent` underline) · "Unazofuata" (`neutral600`) · language chip "SW" (11px, 1px border) · search icon.
- **Item** (padding 60 top / 72 right / 28 bottom / 20 left, vertically centered, gap 14): tag (kind) + source 12px `neutral700`; optional media figure 170 high blueprint-framed; main text Barlow Condensed 600 **30px / 1.14**; parallel English 14px/1.5 `neutral700` (toggleable); reference; optional primary CTA (min-height 44, 16px).
- **Right rail** (right 10, bottom 36, gap 14): Like (count), Hifadhi (save), Sikiliza (listen), Shiriki (share). Label 11px. Liked = heart filled, `accent`.
- Content kinds & kicker labels: `Aya ya Siku`, `Ellen G. White`, `Unabii`, `Shule ya Sabato`, `Nyimbo za Kristo`, `Afya`, `Vijana · Pathfinder`.

### 1b — Feed, "Steel field" (candidate Video tab)
Ground `accent900`, text `bg`. Top tabs centered "Sauti · **Zote** · Video" (17px condensed). Content bottom-aligned; text 36px/1.06; kicker 12px uppercase `accent300`; English `accent200`. Media fills the item. 2px progress bar at bottom (`accent800` track, `bg` fill). **Data saver**: media replaced by a bordered row "Kuokoa data: gusa kupakua video (4.8 MB)".

### 1c — Feed, "Study stack"
Horizontal filter chips (Zote, Biblia, EGW, Shule ya Sabato, Unabii, Nyimbo, Afya). Item = blueprint card: kicker/source + SW|EN segmented toggle; text 26px; reference; "Soma pamoja" list of linked items (rows 44 high, tag + label + chevron). Action row below: 4 icon buttons 44×44 + primary "Jifunze zaidi".

### 1d — Onboarding · language
Wordmark "NENO" (condensed 600, 24px, tracking 0.12em) + "Hatua 1 / 3". H1 40px "Karibu. Chagua lugha yako." Sub 15px. Three radio rows (min-height 60, 1px border; selected = `accent100` fill + `accent` border): Kiswahili (Tanzania · Kenya · Uganda · DRC, "Chaguo-msingi"), English, Français. Section "Pakua kwa matumizi bila mtandao" with checkable packs + sizes. Primary "Endelea" full width, 52 high, 18px, pinned bottom.
Steps 2–3 (not drawn): Bible translation + parallel language; location for sunset (default from device, manual city fallback).

### 1e — Bible reader
Header: back · "Yohana 3" (22px condensed) · `SUV` outline tag · text-size + search icons. Segmented "Kiswahili | Kiswahili + English". Verses 18px/1.6, verse number superscript 13px condensed `accent700`; English under each verse 14px `neutral700`. Selected verse: `accent100` background + action tags (Angazia, Andika, Shiriki, EGW link). Audio bar: 44 primary play, title 13px, 2px progress, time 12px.

### 1f — Library · EGW books
H2 30 "Maktaba" + search. Segmented "Vitabu vya EGW | Imani 28 | Kozi". "Endelea kusoma" blueprint card (title 22px, author line, 2px progress + %). 2-column grid (gap 18×14): cover 112 high, Swahili title 17px condensed, English title 12px, status row (Imepakuliwa ✓ / Pakua · size ↓) 12px `accent700`.

### 1g — Sabbath School
Top band `accent900`: "Sabato inaanza Ijumaa" / "18:43 · Nairobi". Quarter label, H2 lesson title, date range. 7-cell day strip (Sab Jpi Jtt Jnn Jtn Alh Iju): done = `accent100` + ✓, today = `accent` fill. Day title 20px, body 15px/1.6. Memory verse blueprint card (21px condensed). Question label + textarea (`surface` fill, 1px border). Buttons: secondary "Sikiliza somo" + primary "Hifadhi jibu".

### 1h — Hymnal list
H2 "Nyimbo"; search field (44 high, `surface`, search + mic icons) "Tafuta kwa namba au jina"; segmented "Nyimbo za Kristo | SDA Hymnal". Rows 60 high: number 24px condensed `accent700` (width 44), title 15px/500, category 12px, audio/download icon. Mini player band `accent900`.

### 1i — Search
Active field (1px `accent` border) + voice (mic). Filter chips. Grouped results: Biblia, Roho ya Unabii, Imani za Msingi, Video — group label + rows (title 17px condensed, snippet 14px).

### 1j — Profile
Blueprint avatar 60 with initials, name 22px, church line. Streak row (34px number `accent700`). "Vilivyohifadhiwa" list (rows 52). "Mipangilio" rows 48: Lugha, Tafsiri ya pili, Ukubwa wa maandishi, Machweo ya Sabato, Kuokoa data, Upakuaji.

### 2a — EGW book reader
Header: back, book title 20px + "Sura 1 · …" 12px, text-size + bookmark. Paragraph grid: 44px ref column ("SC 9.1", 13px condensed `accent700`) + text 17px/1.6 with optional English 14px. Highlighted paragraph `accent100`. Related verse blueprint card. Bottom: play + "uk. 9 / 126" progress.

### 2b — Fundamental Beliefs (Imani 28)
Same header/segmented as 1f with "Imani 28" selected. Groups with label + range (e.g. "Maisha ya Mkristo 19–23"); rows 52: number 22px condensed, Swahili 15px/500, English 12px, chevron.

### 2c — Course lesson + quiz
Back + "Somo la 3 kati ya 12"; 12-segment progress (4px; done `accent`, current `accent400`, todo `neutral300`). Kicker, H2 30, figure 150, instruction. Multiple-choice rows (min-height 48). On answer: correct row `accent100` + `accent` border + "Sahihi"; wrong picked row shows "Jaribu tena"; reveal a verse card (Danieli 7:17). Pinned primary "Endelea · Somo la 4".

### 2d — Hymn lyrics
Header: number 26px `accent700`, Swahili title + English title, text-size. Stanza tags (Ubeti 1/2/3). Lyrics 26px/1.3 condensed. Audio mode segmented "Kwaya | Piano tu | Bila sauti". Player band `accent900`.

### 2e — Notes & highlights
Chips Zote/Angazo/Maelezo/Nukuu. Entries: tag + ref (16px condensed) + date; quoted text on `accent100`; user note with 1px `accent` left rule, padding-left 10.

### 2f — Sabbath sunset
Top `accent900` block: city, "Sabato inaanza leo, Ijumaa 25 Sep", time **88px** condensed, countdown 15px `accent200`. Reminder row with square toggle (44×26, on = `accent` fill). Lists "Ijumaa zijazo" and "Miji mingine leo" (rows 44, time 19px condensed).

## Interactions & Behavior
- Feed: vertical paging with snap, one item per swipe, `scroll-snap-stop: always` equivalent (`pagingEnabled`, `decelerationRate="fast"`). Preload ±2 items; video autoplays muted only when visible and not in data-saver.
- Like/save: optimistic toggle, queued offline, synced later.
- "Sikiliza": plays item audio (verse/lesson/hymn) with background audio (`expo-av` / `expo-audio`).
- SW/EN toggle and "Kiswahili + English": toggles the parallel language app-wide (persisted setting `parallelLang`).
- Verse long-press → select → action tags (highlight, note, share image, EGW link).
- Quiz: single choice, immediate feedback, correct answer revealed after first pick.
- Sabbath reminder: schedules local notification 60 min before Friday sunset (on-device computation, works offline).
- Data saver: no autoplay, no video prefetch, images low-res, tap-to-load media showing byte size.
- Pressed states: primary → `accent600` (hover) / `accent700` (pressed); secondary/ghost → text @7% / @14% tint. Focus ring 2px `accent`, offset 2.
- Loading: skeleton lines in `neutral200`. Offline: banner "Hakuna mtandao — unatumia maudhui yaliyopakuliwa".

## State Management (client)
- `settings`: uiLang (`sw`|`en`|`fr`), parallelLang (nullable), bibleTranslation (`SUV`), textScale (1.0–2.0), dataSaver, sunsetLocation.
- `feed`: cursor-paginated list, per-item liked/saved.
- `reader`: current osisRef, selected verses.
- `library`: download state per pack (`none|downloading|ready`), reading progress per book/paragraph.
- `userData` (synced): highlights, notes, saves, likes, quiz answers, Sabbath School answers, streak.
- Recommended: TanStack Query for server data, Zustand for settings, `expo-sqlite` for offline content + outbox queue.

## Design Tokens
See `mobile/theme.ts` (authoritative). Summary:
- bg `#f2f2f3`, surface `#e9e9ea`, text `#1d1f20`, accent `#5980a6`, divider = text @16%.
- accent ramp 100 `#eef6ff` · 200 `#d6ebff` · 300 `#b5d9fd` · 400 `#94bce3` · 500 `#749dc4` · 600 `#597ea3` · 700 `#416180` · 800 `#2c455d` · 900 `#1d2d3d`.
- neutral ramp 100 `#f5f5f8` · 200 `#e7e7ea` · 300 `#d4d4d7` · 400 `#b7b7ba` · 500 `#98989b` · 600 `#7a7a7d` · 700 `#5d5d60` · 800 `#424244` · 900 `#2b2b2d`.
- Fonts: Barlow Condensed 600 (headings), Barlow 400/500/700 (body). Heading letter-spacing −0.015em, line-height 1.12.
- Type sizes used: 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 22, 24, 26, 30, 36, 40, 88.
- Spacing scale: 3.4, 6.8, 10.2, 13.6, 20.4, 27.2 (density 0.85); layouts use 4/6/8/10/12/14/16/18/20.
- Radius: 0 for all components.
- Shadows: sm `0 1 2 #2b2b2d@14%`, md `0 3 10 @16%`, lg `0 12 32 @22%` (used only on device frames/dialogs).
- Contrast: body text never in raw `accent` — use `accent700`+.

## Assets
- Icons: Lucide (MIT). No custom illustration; all imagery slots are placeholders (video thumbnails, book covers, prophecy diagrams) to be supplied by the content team.
- Fonts: Google Fonts Barlow / Barlow Condensed (OFL) → `@expo-google-fonts/barlow`, `@expo-google-fonts/barlow-condensed`.
- SDA logo/name use must follow General Conference brand guidelines.

## Content & licensing (blocking for launch)
- Bible: Swahili Union Version (SUV) and any English/French text must be licensed (Bible Society of Kenya/Tanzania, Digital Bible Library) — ingest USFM/USX.
- EGW: Ellen G. White Estate (EGW Writings API), official Swahili/French editions.
- Hymnals: Nyimbo za Kristo lyrics/audio rights; SDA Hymnal.
- Sabbath School: official quarterly via East-Central Africa Division channel.

## Files
- `design/Neno App.dc.html` — all screens on one canvas (open in a browser; needs the sibling `support.js` and `_ds/` folder).
- `design/_ds/.../styles.css` — source design tokens and component CSS.
- `BACKEND.md`, `CLAUDE.md`, `db/001_init.sql`, `mobile/theme.ts`.
