# Снимок Cursor-конфига

Дата: 22 октября 2026, 14:35 (Asia/Yekaterinburg).
Коммит исходников на момент копии: `fd760ed` (`main`).

Это бэкап действующих файлов, не рабочая копия. Cursor читает оригиналы из `.cursor/rules`, `.cursor/skills`, `.cursor/agents`.

## Как восстановить

Скопировать нужные файлы обратно:

```bash
cp -a .cursor/backup/2026-10-22-1435-yekaterinburg/rules/. .cursor/rules/
cp -a .cursor/backup/2026-10-22-1435-yekaterinburg/agents/. .cursor/agents/
cp -a .cursor/backup/2026-10-22-1435-yekaterinburg/skills/. .cursor/skills/
```

## Правила (`rules/`)

- `commit-to-main.mdc`
- `inspector-editing.mdc`
- `merge-to-main.mdc`
- `project.mdc`
- `tz-workflow.mdc`
- `update-user-help.mdc`

## Skills (`skills/`)

- `go-logging/SKILL.md`
- `tz-workflow/SKILL.md`
- `tz-intake/SKILL.md` (этап 1)
- `tz-questions/SKILL.md` (этап 2)
- `tz-solutions/SKILL.md` (этап 3)
- `tz-tasks/SKILL.md` (этап 4)
- `tz-execute/SKILL.md` (этап 5)
- `ui-ux/SKILL.md`
- `user-docs/SKILL.md`

## Субагенты (`agents/`)

- `lead.md`
- `architect.md`
- `designer.md`
- `worker.md`
- `tester.md`
- `ui-tester.md`
- `tech-writer.md`
