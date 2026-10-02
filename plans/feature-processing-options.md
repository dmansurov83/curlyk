# Фича: processing options запроса

## Цель

Параметры запроса захардкожены: `runner.Options{FollowRedirects: false}`
(`app.go:464`), таймаут 30с (`runner.go:15`, `app.go:462`), нет UI для
follow-redirects / insecure TLS / кастомного таймаута. Дать пользователю контроль
через опции в самом `.http`-файле (в стиле JetBrains).

## Дизайн синтаксиса

Строка(и) сразу после request-line, до заголовков/тела:
```
get {{host}}/users
@timeout 5s
@no-redirect
@insecure
```
Либо блок `{% ... %}` Processing Options:
```
{% timeout 5000, follow-redirects: off, insecure %}
```
Выбрать один синтаксис и поддержать единообразно.

## Чек-лист

- [x] Парсер `httpfile`: распознавать строки `@option` / `{% ... %}` в режиме headers
      до первого заголовка/тела; класть в новый слайс `Request.Options`.
- [x] `runner.Options` уже имеет нужные поля (`FollowRedirects`, `Insecure`,
      `Timeout`, `runner.go:38-45`) — складывать их из разобранных опций.
- [x] Приоритет: опция из файла важнее дефолта 30с; дефолты не меняются.
- [x] Поддержать минимум: `timeout`, `redirects` (follow/no-follow), `insecure`.
- [x] Валидация значений (нечисловой таймаут → статус с ошибкой).
- [x] Тесты: парсинг опций, применение к `runner.Options`, дефолтное поведение.
- [x] Обновить README.
- [x] Новые UI-строки выносить ключами в i18n (см. plans/i18n.md).

## Terms of Done

- [x] `@timeout 5s` реально ограничивает запрос 5 секундами.
- [x] `@no-redirect` даёт `CheckRedirect = http.ErrUseLastResponse`.
- [x] `@insecure` подключает `insecureTransport` (`runner.go:30-34`).
- [x] `go test ./...` и `go build -o bin/curlyk.exe ./cmd/curlyk` проходят.