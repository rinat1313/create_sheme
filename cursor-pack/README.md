# Пакет Cursor для переноса в другой проект

Снимок важных файлов Cursor из `create_sheme`: skills, rules, subagents.

| Папка | Откуда | Куда класть в новом проекте |
|-------|--------|-----------------------------|
| `skills/` | `.cursor/skills/` | `.cursor/skills/` |
| `rules/` | `.cursor/rules/` | `.cursor/rules/` |
| `agents/` | `.cursor/agents/` | `.cursor/agents/` |

- `user-chat-history.md` — исходные формулировки из чата по этому проекту (что писал пользователь).
- `catalog.md` — кратко, что делает каждый skill, rule и subagent.

## Как добавить в другой проект

Из корня **нового** репозитория:

```bash
mkdir -p .cursor && cp -a путь/к/create_sheme/cursor-pack/skills .cursor/skills && cp -a путь/к/create_sheme/cursor-pack/rules .cursor/rules && cp -a путь/к/create_sheme/cursor-pack/agents .cursor/agents
```

После копирования коммит в `main` нового проекта (правила пакета сами требуют актуальный `.cursor/` в `origin/main`).
