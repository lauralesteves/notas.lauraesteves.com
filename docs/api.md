# API de notas

Contrato entre `frontend/` e `backend/`. Qualquer mudança aqui precisa ser feita nos dois lados.

## Endereços

| Ambiente | HTTP | WebSocket | Raw (curl) |
|-|-|-|-|
| Produção | `https://api.notas.lauraesteves.com` | `wss://ws.notas.lauraesteves.com` | `https://notas.lauraesteves.com/raw/...` (CloudFront -> HTTP API) |
| Local (`make backend-dev`) | `http://localhost:8080` | `ws://localhost:8080/ws` | `http://localhost:8080/raw/...` |

Frontend lê `VITE_API_URL` e `VITE_WS_URL` (padrão: os valores locais).

## Caminhos de nota

Regra única, validada no backend e normalizada no frontend antes de chamar a API:

- Segmentos separados por `/`, cada um casando `^[a-z0-9_-]{1,64}$`.
- Máximo de **5 níveis** e **200 caracteres** no total.
- Sem barra no começo ou no fim. Raiz vazia não é nota.
- O primeiro segmento não pode ser reservado (nomes com ponto já são barrados pela regra do segmento e ficam na lista só como documentação): `api`, `ws`, `raw`, `notes`, `children`, `assets`, `static`, `favicon.ico`, `robots.txt`, `sitemap.xml`, `manifest.webmanifest`, `humans.txt`, `ai.txt`, `well-known`, `index.html`, `404`.
- Frontend normaliza o que a pessoa digitou na URL: minúsculas, remove acentos (NFD), espaços viram `-`, remove o resto inválido, junta barras repetidas. Se mudou, troca a URL com `history.replaceState`.

## Limites

- Corpo da nota: **100.000 bytes** (UTF-8). Acima disso: `413`.
- Nota expira **15 dias** depois da última edição (`expiresAt`). Nota vencida é tratada como inexistente em toda leitura.
- Corpo vazio no save **apaga** a nota.
- Paths já precisam chegar normalizados: o backend valida e recusa, não corrige.

## Formato comum

Datas em ISO 8601 UTC (`2026-11-22T18:42:00Z`). Erros sempre como:

```json
{"error": "invalid_path", "message": "texto para log"}
```

Códigos de `error`: `invalid_path`, `invalid_body`, `too_large`, `conflict`, `not_found`, `rate_limited`, `internal`.

Quando o API Gateway limita requisições, ele responde `429` com o corpo dele (`{"message":"Too Many Requests"}`). O cliente trata **qualquer `429`** como `rate_limited`, independente do corpo.

Objeto `Note`:

```json
{
  "path": "faculdade/calculo",
  "body": "texto",
  "version": 8,
  "updatedAt": "2026-11-22T18:42:00Z",
  "expiresAt": "2026-12-07T18:42:00Z"
}
```

Nota inexistente ou vencida: `{"path": "...", "body": "", "version": 0, "updatedAt": null, "expiresAt": null}`.

## HTTP

### `GET /notes/{path}`

`200` com `Note`. `400 invalid_path`.

### `GET /notes/{path}?since={version}`

Polling. Se a versão atual for igual a `since`: `200 {"changed": false, "version": N}`. Senão: `200` com `Note` e `"changed": true`.

### `PUT /notes/{path}`

Headers: `Content-Type: application/json`, opcional `X-Connection-Id` (o broadcast pula essa conexão).

```json
{"body": "texto", "baseVersion": 7, "force": false}
```

- `200 {"path", "version", "updatedAt", "expiresAt"}`. Se o corpo veio vazio e a nota foi apagada: `version: 0`, datas `null`.
- `409 {"error": "conflict", "message": "...", "note": Note}` quando `baseVersion` não é a versão atual e `force` não é `true`. Nota vencida ou inexistente aceita qualquer `baseVersion` e recomeça em `version: 1`.
- `400 invalid_path | invalid_body`, `413 too_large`.

### `GET /children/{path}`

Subpáginas diretas, não vencidas, em ordem alfabética. (Existe no backend, mas a interface atual não usa: a página da nota é só a nota.)

```json
{"path": "faculdade", "children": [{"path": "faculdade/calculo", "updatedAt": "..."}]}
```

### `GET /raw/{path}`

`200 text/plain; charset=utf-8` com o corpo e um `\n` no final se não terminar com um. `404` (texto `not found\n`) se não existe ou venceu.

### `PUT /raw/{path}`

Corpo `text/plain`. Last-write-wins (equivale a `force: true`). `200` texto `saved v{N}, expires {expiresAt}\n`; corpo vazio apaga e responde `deleted\n`. Erros em texto: `invalid path\n`, `invalid body\n`, `too large\n`. Mesmos limites e mesmo broadcast.

### CORS

Origens: `https://notas.lauraesteves.com`, `http://localhost:5173`. Métodos `GET, PUT, OPTIONS`. Headers `Content-Type, X-Connection-Id`.

## WebSocket

Conexão: `{WS_URL}?path={path}`. Path inválido: conexão recusada (`400`). Mais de **50 conexões** na mesma nota: recusada (`429`), e o cliente usa polling.

### Cliente -> servidor

```json
{"action": "ping"}
```

Enviar logo ao abrir (para descobrir o próprio `connectionId`) e depois a cada **5 minutos** (o API Gateway derruba após 10 min sem tráfego). A conexão cai em no máximo 2h: o cliente reconecta com backoff (1s, 2s, 4s... até 30s) e faz um `GET /notes/{path}` ao reconectar.

### Servidor -> cliente

```json
{"type": "pong", "connectionId": "abc=", "count": 2}
{"type": "update", "path": "foo", "version": 8, "body": "...", "updatedAt": "...", "expiresAt": "..."}
{"type": "changed", "path": "foo", "version": 8}
{"type": "presence", "path": "foo", "count": 3}
```

- `update` quando o corpo tem até **28.000 bytes**; acima disso vai `changed` e o cliente faz `GET`.
- Nota apagada (corpo vazio): `update` com `version: 0`, `body: ""`, datas `null`.
- `presence` vai para os outros da mesma nota quando alguém entra, sai ou manda `ping` (o reenvio no `ping` corrige contagens erradas por causa do índice eventualmente consistente do DynamoDB). `count` inclui todo mundo conectado na nota.
- Se o WebSocket não conectar, o cliente cai para polling (`GET ?since=`) a cada 5s, só com a aba visível.
