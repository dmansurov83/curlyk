---
name: curlyk-release
description: Выпускает GitHub-релиз проекта curlyk (Go TUI-клиент HTTP) через GitHub Action — «сделай релиз», «сделай следующий релиз / release», «залей новую версию», «выпусти vX», "make a release", "create a github release", "bump the version and release". Достаточно определить следующую версию тега v0.0.x, создать git-тег и запушить его: GitHub Action .github/workflows/release.yml сам соберёт бинарники Windows amd64/arm64, упакует их в zip-ассеты curlk-windows-*.zip, создаст релиз с Release notes (--generate-notes) и загрузит ассеты. Локально ничего собирать не нужно. Do NOT use для plain git-коммитов, для локальной сборки без публикации, для релиза других продуктов — там обычная работа по сборке/коммитам.
---

# Выпуск GitHub-релиза проекта curlyk через GitHub Action

Релизы выпускаются АВТОМАТИЧЕСКИ через CI. Владелец репозитория должен только
создать и запушить git-тег со следующей версией. Ничего локально собирать и
паковать не нужно.

## Как это работает

При push любого тега `v*` (в `.github/workflows/release.yml`) выполняется:

1. **Create release** — `gh release create --generate-notes --verify-tag`:
   создаёт релиз и сам пишет Release notes из commits/PR.
2. **Build** (matrix `windows/amd64`, `windows/arm64`) — кросс-компиляция на
   ubuntu: `go build -trimpath -ldflags "-s -w" -o curlyk-windows-<arch>.exe ./cmd/curlyk`.
3. **Zip** — `curlk-windows-<arch>.zip` (внутри exe назван `curlyk-windows-<arch>.exe`).
4. **Upload asset** — грузит zip как ассет релиза (`--clobber`).

Go 1.27 ставится `actions/setup-go@v5`, `CGO_ENABLED=0`.

## Важное правило: никогда не смешивать ручной и CI-путь

`gh release create` в CI падает, если на теге УЖЕ существует релиз
(`a release with the same tag already exists`). Поэтому:

- НЕ создавать релиз вручную (`gh release create`/`edit`) перед тем как запушить
  тег, на который ещё не было релиза.
- Если тег зарелизен вручную и хочешь дальше через CI — исправлять не нужно, оба
  пути допустимы, но для нового тега выбирай ОДИН.

## Условия

- Рабочее дерево чистое (`git status -sb`) — если есть незакоммиченные
  изменения, остановись и спроси.
- Есть доступ на push к `origin` → `https://github.com/dmansurov83/curlyk.git`.

## Шаг 1. Определи следующую версию

Последний выпущенный тег — это следующая версия по схеме существующих `v0.0.x`:

```
git tag --sort=-v:refname
```

Сортировка `--sort=-v:refname` важна: сравнивает как *версии*, иначе `v0.0.10`
встанет раньше `v0.0.2`. Возьми самый высокий тег и прибавь единицу к младшей
позиции. Если есть `v0.0.2` и `v0.0.3`, следующая — `v0.0.4`.

## Шаг 2. Создай и запуши тег — и всё

```
git tag v0.0.X && git push origin v0.0.X
```

Запушь ТОЛЬКО тег (не ветку, не `--tags`). Push тега запускает workflow
`Release`, который сам создаст релиз с notes и ассетами.

Azure: не ждать завершения action локально без необходимости — пользователь
отслеживает статус на GitHub (вкладка Actions → Release).

## Шаг 3. Проверь итог

После того как workflow отработал (дай ему пару минут или сверь с вкладкой
Actions), релиз должен быть без draft и prerelease, с обоими ассетами:

```
gh release view v0.0.X --json name,isDraft,isPrerelease,assets --jq '{name:.name,draft:.isDraft,prerelease:.isPrerelease,assets:[.assets[].name]}'
```

Дай пользователю ссылку на релиз:
`https://github.com/dmansurov83/curlyk/releases/tag/v0.0.X`.

## Если action упал

- «Create release» упал с `a release with the same tag already exists` — на теге
  уже был релиз (например создан вручную ранее). Это ожидаемо, а не ошибка CI:
  выпуск уже состоялся, CI-часть прогнать нельзя без удаления релиза.
- Сбой сборки/аплоада — смотри логи шага на вкладке Actions, исправь, но помни:
  `gh release create` выполнился в 1-м шаге, поэтому повторный пуск того же тега
  упрётся в «already exists». Правильный повтор — удалить созданный релиз
  (`gh release delete <tag>`) и переписать тег, либо заливать ассеты вручную
  через `gh release upload`.