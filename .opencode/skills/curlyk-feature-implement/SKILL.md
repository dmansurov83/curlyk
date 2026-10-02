---
name: curlyk-feature-implement
description: Реализует запланированную фичу проекта curlyk (Go TUI-клиент HTTP) по плану из `plans/` — «реализуй фичу», «сделай фичу», «внедри план feature-profiles», «реализуй фичу из плана», "implement the feature", "build feature X from the plan". Проходит чек-лист и Terms of Done плана: пишет Go-код в internal/ (tui, httpfile, curl, runner, settings, i18n), покрывает тестами, прогоняет `go test ./...` и `go build -o bin/curlyk.exe ./cmd/curlyk`, переносит план в `plans/completed/` и коммитит. Do NOT use для черновика плана без реализации (это curlyk-feature-plan), для разовых bug-фиксов без плана, для верстки документов.
---

# Реализация фичи по плану в проекте curlyk

Внедряешь фичу, описанную в `plans/feature-<slug>.md`. Сначала прочитай план
полностью и подтверди его: если пункты расплывчаты — сначала уточни у
пользователя, потом кодируй.

## Архитектура проекта (куда что писать)

- `cmd/curlyk/main.go` — точка входа; `rsrc_windows_amd64.syso` — Windows-ресурс.
- `internal/tui/` — TUI (bubbletea), МОДУЛЬ раскидан по файлам:
  - `app.go` — структура `model`, Args/New, run/applyResponse, initialModel.
  - `keys.go` — обработка клавиш, хоткеи (`handleKey`/`handleEditKey`), там же
    батч-вставка и `toggleLanguage` (i18n, переключает язык и пишет в appsettings).
  - `render.go`, `render_editor.go`, `render_resp.go` — рендер панелей.
  - `actionmenu.go` — всплывающее меню действий.
  - `session.go` — автосейв, dirty-флаг, позиция курсора.
- `internal/httpfile/` — парсер .http (lexer/parser), переменные (`vars.go`).
- `internal/curl/` — импорт cURL (import.go), маппинг в HTTP (`to_http.go`).
- `internal/runner/` — выполнение запросов.
- `internal/settings/` — `settings.go` + `appsettings.yml` (персист).
- `internal/i18n/` — локали ru/en, `T(key, args...)`, выбор CURLYK_LANG→settings→ru.

## ВАЖНЫЕ правила проекта

1. **Туръю консольная**: бинарник собирать БЕЗ `-H=windowsgui`
   (`go build -o bin/curlyk.exe ./cmd/curlyk`), иначе TUI запускается без консоли.
2. **Каждую новую UI-строку выносить ключом в i18n** (`internal/i18n/`, локали
   ru и en) и использовать `T(...)`, а не захардкоженный текст.
3. Тесты рядом с кодом (`*_test.go`) обязательны для новой логики. Смотри
   существующие тесты (`internal/tui/*_test.go`, `internal/httpfile/*_test.go`)
   как образец стиля.
4. Код по `gofmt`; в этом репозитории файлы с CRLF, поэтому `gofmt -l` может
   флайгать чистые файлы — проверяй конкретный файл после `gofmt -w`.

## Процесс

1. Прочитай `plans/feature-<slug>.md`.
2. Изучи релевантный код (см. архитектуру выше), уточни неоднозначности у
   пользователя, если они есть.
3. Отмечай пункты чек-листа плана `- [x]` по мере выполнения.
4. Пиши код + тесты.
5. Прогони:
   - `go test ./...`
   - `go build -o bin/curlyk.exe ./cmd/curlyk`
   Всё должно быть зелёным, иначе фича не принята.
6. Отметь Terms of Done плана `- [x]`.
7. Перенеси план в `plans/completed/` (стиль коммита проекта `f4b9480`:
   `git mv plans/feature-<slug>.md plans/completed/`).
8. Коммить ТОЛЬКО если пользователь явно попросил. Стиль commit message как в
   истории: `Add <feature> …` или `<feature> completed.` (см. `git log --oneline`).

## Accept criteria

- `go test ./...` зелёный, `go build -o bin/curlyk.exe ./cmd/curlyk` проходит.
- Все пункты чек-листа и Terms of Done плана `- [x]`.
- План лежит в `plans/completed/`.
- Коммит сделан только по просьбе пользователя.