# Nginx: большие загрузки solution XML

При размещении API за внешним reverse-proxy (nginx) лимиты и таймауты proxy должны позволять передачу файлов до ~1 ГБ (Solution.xml или .rar с ZIP внутри) и длительный ответ клиента после upload (клиент сразу получает `202`, но сам POST upload может идти долго).

## Рекомендуемые директивы

```nginx
client_max_body_size 1g;

proxy_connect_timeout 60s;
proxy_send_timeout 3600s;
proxy_read_timeout 3600s;

client_body_timeout 3600s;
send_timeout 3600s;

proxy_request_buffering on;
proxy_buffering on;
```

`client_max_body_size` должен быть не меньше лимита приложения (`IMPORT_MAX_BYTES`, по умолчанию 1 ГиБ). Иначе nginx вернёт `413` до попадания запроса в API.

Большие `proxy_*_timeout` нужны на этап передачи тела запроса. После spool API отвечает быстро (`202 Accepted`); парсинг идёт в фоне.

## Пример `location /api/`

```nginx
server {
    listen 443 ssl;
    server_name example.com;

    client_max_body_size 1g;
    client_body_timeout 3600s;

    location /api/ {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_connect_timeout 60s;
        proxy_send_timeout 3600s;
        proxy_read_timeout 3600s;
        send_timeout 3600s;

        proxy_request_buffering on;
        proxy_buffering on;
    }

    location / {
        proxy_pass http://127.0.0.1:3001;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Проверка

1. Загрузить XML ~400–600 МБ через UI или `curl -F file=@big.xml http://host/api/v1/solutions`.
2. Ожидать быстрый `202` с `"status":"importing"`, затем poll `GET /api/v1/solutions/{id}` до `ready` / `failed`.
3. При файле больше лимита — `413` от nginx или API.
