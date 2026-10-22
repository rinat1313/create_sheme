# Каталог skills, rules и subagents

Снимок на момент коммита пакета. Подробности — в файлах папок `skills/`, `rules/`, `agents/`.

## Skills

Главный skill — `tz-workflow`: цепочка из 8 этапов. Остальные этапные skills его дополняют.

| Skill | Этап / роль |
|-------|-------------|
| `tz-workflow` | вся цепочка ТЗ |
| `tz-intake` | 1. Получение ТЗ |
| `tz-questions` | 2. Вопросы |
| `tz-solutions` | 3. Решения |
| `tz-tasks` | 4. Задачи |
| `tz-execute` | 5. Выполнение |
| `tz-verify` | 6. Проверка |
| `tz-docs` | 7. Документация |
| `tz-report` | 8. Отчёты |
| `tz-specialists` | группа Специалисты (маршруты субагентов) |
| `tz-executors` | группа Исполнители |
| `tz-testers` | группа Тестировщики |
| `tz-writers` | группа Технические писатели |
| `ui-ux` | дополнение для frontend/UI |
| `user-docs` | стиль руководства и help |
| `go-logging` | логирование функций в Go |

## Rules

| Rule | Зачем |
|------|--------|
| `tz-workflow.mdc` | всегда 8 этапов, вызов lead и групп |
| `project.mdc` | стек, help, схемы на фронт, коммит в main |
| `commit-to-main.mdc` | коммит готовой работы в `main` |
| `merge-to-main.mdc` | merge feature-ветки в `main` |
| `cursor-config-on-main.mdc` | актуальный `.cursor/` только в `origin/main` |
| `update-user-help.mdc` | обновлять help и руководство при смене UI/логики |
| `inspector-editing.mdc` | правила инспектора схем (специфика этого продукта) |

## Subagents

Координатор: `lead`. Остальные вызываются lead через Task по зоне.

**Специалисты:** `analyst`, `architect`, `ui-ux-designer`, `security`, `algorithmist`, `programmer`, `database`.

**Исполнители:** `backend`, `frontend`, `sql`, `devops`.

**Тестировщики:** `qa-tester`, `manual-tester`, `sql-tester`.

**Технические писатели:** `technical-writer`.
