# API contract

[`openapi.yaml`](openapi.yaml) — единственный источник HTTP DTO и error shapes для Mixologist.

Go server stubs **не** генерируются. Backend реализует handlers вручную; соответствие контракту проверяется contract-тестами (`kin-openapi`) на реальных HTTP-ответах.

## Hybrid pipeline

```text
api/openapi.yaml
      │
      ├─► backend handlers (ручная реализация под `/api/v1`)
      │         │
      │         └─► contract tests (kin-openapi валидирует ответы)
      │
      └─► openapi-typescript
                │
                └─► frontend/src/shared/api/generated.ts
                          │
                          └─► frontend API client / feature types
```

1. **Spec** — правим `api/openapi.yaml`.
2. **Backend** — обновляем handlers/DTO под контракт (без codegen Go stubs).
3. **Contract tests** — integration-тесты валидируют реальные ответы по OpenAPI.
4. **TS generation** — `openapi-typescript` обновляет `frontend/src/shared/api/generated.ts`.
5. **Frontend** — использует generated request/response DTO; формы и UI-модели остаются ручными.

## Команды

Запускаются из `frontend/` (зависимости установлены локально через npm):

```bash
npm run api:lint      # Redocly: валидация и lint spec
npm run api:generate  # обновить generated.ts
npm run api:check     # generate во временный файл и fail при diff
```

Generated-файл хранится в репозитории. После изменения spec всегда коммитьте обновлённый `generated.ts` вместе со spec.

> `openapi-typescript` пока объявляет peer `typescript@^5`; в `frontend/.npmrc` включён
> `legacy-peer-deps=true`, чтобы установка работала с TypeScript 6 проекта.
> Из‑за этого peer `@testing-library/dom` указан явно в `devDependencies`.

## Версионирование и границы

- Продуктовый API: `/api/v1/...`
- Operational health: `GET /health` (вне `/api/v1`)
- Cookie session security scheme описан в spec заранее; enforcement — этап 1
