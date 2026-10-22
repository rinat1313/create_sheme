# Пакет Cursor для переноса в другой проект

Снимок skills, rules и subagents из `create_sheme` (`origin/main`).

**Главный файл для нового агента:** [`FOR-NEW-AGENT.md`](FOR-NEW-AGENT.md)  
Развёрнутая инструкция: как скопировать пакет, когда запускать ТЗ, 8 этапов, кого вызывать, модели, git, текст первого сообщения.

| Папка / файл | Куда |
|--------------|------|
| `skills/` | `.cursor/skills/` |
| `rules/` | `.cursor/rules/` |
| `agents/` | `.cursor/agents/` |
| `FOR-NEW-AGENT.md` | оставь в `cursor-pack/` или положи в корень нового репо и дай агенту в чат |
| `catalog.md` | краткий перечень |
| `user-chat-history.md` | исходные формулировки человека из треда (архив, не конфиг Cursor) |

## Как добавить в другой проект

```bash
mkdir -p .cursor
cp -a cursor-pack/skills .cursor/skills
cp -a cursor-pack/rules .cursor/rules
cp -a cursor-pack/agents .cursor/agents
```

После копирования закоммить в `main` нового проекта. Правило `inspector-editing.mdc` можно не брать, если нет инспектора схем.
